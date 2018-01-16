package client

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	k8ccv1alpha1 "github.com/mbrt/k8cc/pkg/apis/k8cc.io/v1alpha1"
	clientset "github.com/mbrt/k8cc/pkg/client/clientset/versioned"
	k8ccscheme "github.com/mbrt/k8cc/pkg/client/clientset/versioned/scheme"
	listers "github.com/mbrt/k8cc/pkg/client/listers/k8cc/v1alpha1"
	"github.com/mbrt/k8cc/pkg/kube"
)

const controllerAgentName = "k8cc-distcc-client-controller"

// Controller is responsible for DistccClient and DistccClientClaim objects management
type Controller interface {
	Run(threadiness int, stopCh <-chan struct{}) error
}

// NewController creates a new Controller
func NewController(
	sharedClient *kube.SharedClient,
	logger log.Logger,
) Controller {
	podInformer := sharedClient.KubeInformerFactory.Core().V1().Pods()
	distccInformer := sharedClient.DistccInformerFactory.K8cc().V1alpha1().DistccClients()

	// Create event broadcaster
	// Add distcc types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	k8ccscheme.AddToScheme(scheme.Scheme)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(func(format string, args ...interface{}) {
		_ = logger.Log("event", fmt.Sprintf(format, args...))
	})
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: sharedClient.KubeClientset.CoreV1().Events("")})

	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	op := controller{
		distccsLister: distccInformer.Lister(),
		distccsSynced: distccInformer.Informer().HasSynced,
		podsLister:    podInformer.Lister(),
		podsSynced:    podInformer.Informer().HasSynced,
		logger:        logger,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "k8cc-distcc-client"),
		recorder:      recorder,
	}

	// Set up an event handler for when Distcc resources change
	distccInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: op.enqueueDistccClient,
		UpdateFunc: func(old, new interface{}) {
			// we ignore if old == new. we take advantage of periodic
			// updates to manage downscaling periodically
			op.enqueueDistccClient(new)
		},
		DeleteFunc: op.enqueueDistccClient,
	})

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: op.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newPod := new.(*corev1.Pod)
			oldPod := old.(*corev1.Pod)
			if newPod.ResourceVersion == oldPod.ResourceVersion {
				// Periodic resync will send update events for all known Pods.
				// Two different versions of the same StatefulSet will always have different RVs.
				return
			}
			op.handleObject(new)
		},
		DeleteFunc: op.handleObject,
	})

	return &op
}

type controller struct {
	kubeclientset      kubernetes.Interface
	k8ccclientset      clientset.Interface
	podsLister         corelisters.PodLister
	podsSynced         cache.InformerSynced
	distccsLister      listers.DistccClientLister
	distccsSynced      cache.InformerSynced
	distccclaimsLister listers.DistccClientClaimLister
	distccclaimsSynced cache.InformerSynced
	logger             log.Logger

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

func (c *controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	if ok := cache.WaitForCacheSync(stopCh, c.podsSynced, c.distccsSynced, c.distccclaimsSynced); !ok {
		return errors.New("failed to wait for caches to sync")
	}

	// Launch workers to process StatefulSet resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *controller) processNextWorkItem() bool {
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
			runtime.HandleError(errors.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Distcc resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return errors.Wrap(err, fmt.Sprintf("error syncing '%s'", key))
		}

		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two.
func (c *controller) syncHandler(key string) error {
	_ = c.logger.Log("method", "syncHandler", "key", key)

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(errors.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the DistccClientClaim resource with this namespace/name
	claim, err := c.distccclaimsLister.DistccClientClaims(namespace).Get(name)
	if err != nil {
		// The Distcc resource may no longer exist, in which case we stop
		// processing.
		if kubeerr.IsNotFound(err) {
			runtime.HandleError(errors.Errorf(
				"distcc client claim '%s' in work queue no longer exists",
				key))
			return nil
		}
		return err
	}

	// Get the corresponding DistccClient resource
	dclient, err := c.distccsLister.DistccClients(namespace).Get(claim.Spec.DistccClientName)
	if err != nil {
		if kubeerr.IsNotFound(err) {
			// The referring DistccClient is invalid, stop processing this element
			runtime.HandleError(errors.Errorf(
				"distccclient '%s' is not present",
				claim.Spec.DistccClientName))
		}
		return err
	}

	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	claim = claim.DeepCopy()

	// If the expiration time is not set, set it automatically
	updated := c.updateExpiration(claim, dclient)

	// Check expiration time, and delete
	if c.isExpired(claim, dclient) {
		return c.k8ccclientset.K8ccV1alpha1().DistccClientClaims(namespace).Delete(name, nil)
	}

	// Check the related POD

	if updated {
		_, err = c.k8ccclientset.K8ccV1alpha1().DistccClientClaims(namespace).Update(claim)
	}

	return err
}

// enqueueDistccClient takes a DistccClient resource and converts it into a
// namespace/name string which is then put onto the work queue. This method
// should *not* be passed resources of any type other than DistccClient.
func (c *controller) enqueueDistccClient(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the DistccClient resource that 'owns' it. It does this by looking at
// the objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that DistccClient resource to be processed. If the object
// does not have an appropriate OwnerReference, it will simply be skipped.
func (c *controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(errors.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(errors.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		_ = c.logger.Log("method", "handleObject", "recovered tombstone", object.GetName())
	}
	_ = c.logger.Log("method", "handleObject", "processing", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Distcc, we should not do anything more
		// with it.
		if ownerRef.Kind != "DistccClient" {
			return
		}

		distcc, err := c.distccsLister.DistccClients(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			_ = c.logger.Log("method", "handleObject", "err", err.Error(), "info",
				fmt.Sprintf("ignore orphaned %s with owner %s", object.GetSelfLink(), ownerRef.Name))
			return
		}

		c.enqueueDistccClient(distcc)
		return
	}
}

func (c *controller) updateExpiration(claim *k8ccv1alpha1.DistccClientClaim, client *k8ccv1alpha1.DistccClient) bool {
	now := time.Now()
	// If expiration is not set, set it automatically
	if claim.Status.ExpirationTime == nil {
		claim.Status.ExpirationTime = toKubeTime(now)
		return true
	}
	// If the expiration exceedes the maximum possible one (namely now + expiration time)
	// then reduce it to that value
	maxExpiration := now.Add(client.Spec.LeaseDuration.Duration)
	if claim.Status.ExpirationTime.After(maxExpiration) {
		claim.Status.ExpirationTime = toKubeTime(maxExpiration)
		return true
	}
	return false
}

func (c *controller) isExpired(claim *k8ccv1alpha1.DistccClientClaim, client *k8ccv1alpha1.DistccClient) bool {
	return claim.Status.ExpirationTime.After(time.Now())
}

func toKubeTime(t time.Time) *metav1.Time {
	// It's necessary to truncate nanoseconds to allow correct comparison
	r := metav1.NewTime(t).Rfc3339Copy()
	return &r
}
