package kit

// see https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
// and https://github.com/kubernetes/sample-controller/blob/master/controller.go
// and https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/podautoscaler/horizontal.go

import (
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	k8ccscheme "github.com/mbrt/k8cc/pkg/client/clientset/versioned/scheme"
	sharedctr "github.com/mbrt/k8cc/pkg/controller"
	k8ccerr "github.com/mbrt/k8cc/pkg/errors"
)

// ControllerKit abstracts away some of the boring details from a controller
// implementation.
//
// It allows to control one type of CustomResourceDefinition and trigger the
// synchronization after a change on a resource, a controlled resource, or
// periodically.
type ControllerKit interface {
	sharedctr.Controller
	// EventRecorder returns the event recorded that can be used to
	// record events on the controlled objects.
	EventRecorder() record.EventRecorder
}

// NewController creates a new controller for a single CustomResourceDefinition.
//
// The given arguments determine how and on what the controller should operate.
// The handler provides the hooks that will be called at the right time, during the
// control loop. The kubeclientset needs to be a valid connection to Kubernetes.
// The crdInformer is a shared informer for the CustomResourceDefinition that this
// controller should manage. The controlledObjectsInformers list is a list of
// informers for objects that can be owned by the CRD. Whenever these objects are
// owned by the given CRD and they change, they will trigger a Sync for the owner
// CRD. The needPeriodicResync option states whether a periodic Sync is needed
// for the CRDs even if they didn't change.
func NewController(
	handler ControllerHandler,
	kubeclientset kubernetes.Interface,
	crdInformer cache.SharedInformer,
	controlledObjectsInformers []cache.SharedInformer,
	needPeriodicResync bool,
) ControllerKit {
	// Create event broadcaster
	// Add the CRD types to the default Kubernetes Scheme so Events can be
	// logged for these CRD types.
	k8ccscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{
		Interface: kubeclientset.CoreV1().Events(""),
	})

	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: handler.AgentName()})

	informersSynced := make([]cache.InformerSynced, len(controlledObjectsInformers)+1)
	for i, informer := range controlledObjectsInformers {
		informersSynced[i] = informer.HasSynced
	}
	op := controllerKit{
		handler:         handler,
		informersSynced: informersSynced,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "k8cc-distcc"),
		recorder:        recorder,
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when the CRD resources change
	crdInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: op.enqueueCustomResource,
		UpdateFunc: func(old, new interface{}) {
			// if the controller needs periodic resync we ignore the old == new check.
			// we take advantage of periodic updates and pass them on to the controller.
			newObj := new.(*corev1.ObjectMeta)
			oldObj := old.(*corev1.ObjectMeta)
			if needPeriodicResync || newObj.ResourceVersion != oldObj.ResourceVersion {
				op.enqueueCustomResource(new)
			}
		},
		DeleteFunc: op.enqueueCustomResource,
	})

	// Set up an event handler for when dependent resources change. This
	// way, we don't need to implement custom logic for handling these
	// resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	for _, informer := range controlledObjectsInformers {
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    op.handleObject,
			UpdateFunc: op.handleObjectUpdate,
			DeleteFunc: op.handleObject,
		})
	}

	return &op
}

// ControllerHandler provides the hooks necessary to the ControllerKit, in order to implement
// a fully functional controller.
type ControllerHandler interface {
	// AgentName returns a unique name for the controller.
	AgentName() string
	// Sync is called whenever a CRD object needs to be synced.
	//
	// The key of the object will be passed as argument. If something goes wrong during
	// the sync, an error needs to be returned. If that error is caused by a transient
	// cause (e.g. temporary network failure), make sure you return a TransientError, so
	// the same object will be retried without waiting for a change.
	Sync(key string) error
	// CustomResourceKind returns the kind of the controlled resource, as it is specified
	// by the CRD manifest.
	CustomResourceKind() string
	// CustomResourceInstance returns the instance of the CustomResourceDefinition with
	// the given key (namespace, name).
	CustomResourceInstance(namespace, name string) (runtime.Object, error)
}

// controllerKit abstracts away some tedous details over implementing a Kubernetes controller
type controllerKit struct {
	handler         ControllerHandler
	informersSynced []cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *controllerKit) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	kind := c.handler.CustomResourceKind()
	glog.Infof("Starting %s controller", kind)

	// Wait for the caches to be synced before starting workers
	glog.Infof("Waiting for %s informer caches to sync", kind)
	if ok := cache.WaitForCacheSync(stopCh, c.informersSynced...); !ok {
		return errors.New("failed to wait for caches to sync")
	}

	glog.Infof("Starting %s workers", kind)
	// Launch workers to process Deployment resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Infof("Started %s workers", kind)
	<-stopCh
	glog.Infof("Shutting down %s workers", kind)

	return nil
}

func (c *controllerKit) EventRecorder() record.EventRecorder {
	return c.recorder
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *controllerKit) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *controllerKit) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(errors.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Distcc resource to be synced.
		if err := c.handler.Sync(key); err != nil {
			// If an error is transient, we'll requeue the item so we can attempt
			// processing again later. This could have been caused by a
			// temporary network failure, or any other transient reason.
			// Otherwise we just fail permanently and wait for the object to change,
			// before to attempt processing it again.
			if k8ccerr.IsTransient(err) {
				return err
			}
			// If an error has an event attached, we'll record it.
			if evt, ok := HasEvent(err); ok {
				c.recorder.Event(evt.Object, evt.Type, evt.Reason, evt.Message)
			}
			utilruntime.HandleError(err)
			return nil
		}

		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// enqueueCustomResource takes the controlled CRD and converts it into a
// namespace/name string which is then put onto the work queue. This method
// should *not* be passed resources of any type other than the expected CRD.
func (c *controllerKit) enqueueCustomResource(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Distcc resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Distcc resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *controllerKit) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(errors.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(errors.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		glog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	glog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Distcc, we should not do anything more
		// with it.
		if ownerRef.Kind != c.handler.CustomResourceKind() {
			return
		}

		crd, err := c.handler.CustomResourceInstance(object.GetNamespace(), ownerRef.Name)
		if err != nil {
			glog.V(4).Infof("ignoring orphaned object '%s' of foo '%s'",
				object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueCustomResource(crd)
		return
	}
}

func (c *controllerKit) handleObjectUpdate(old, new interface{}) {
	newObj := new.(*corev1.ObjectMeta)
	oldObj := old.(*corev1.ObjectMeta)
	if newObj.ResourceVersion == oldObj.ResourceVersion {
		// Periodic resync will send update events for all known objects that could be
		// referenced by one of our Custom Resources.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}
	c.handleObject(new)
}

// Event contains the information to create an Event attached to a kubernetes object.
type Event struct {
	Object  runtime.Object
	Type    string
	Reason  string
	Message string
}

// HasEvent returns the Event attached to the given error, if present
func HasEvent(err error) (Event, bool) {
	te, ok := errors.Cause(err).(eventful)
	if !ok {
		return Event{}, false
	}
	return te.HasEvent()
}

// EventfulError wraps the given error and attaches an Event to it
func EventfulError(err error, event Event) error {
	return eventfulError{err, event}
}

type eventful interface {
	HasEvent() (Event, bool)
}

type eventfulError struct {
	error
	event Event
}

func (e eventfulError) Error() string           { return e.error.Error() }
func (e eventfulError) HasEvent() (Event, bool) { return e.event, true }
