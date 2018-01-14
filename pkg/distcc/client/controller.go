package client

import (
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

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
		DeleteFunc: op.deleteDistccClient,
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
	kubeclientset kubernetes.Interface
	k8ccclientset clientset.Interface
	podsLister    corelisters.PodLister
	podsSynced    cache.InformerSynced
	distccsLister listers.DistccClientLister
	distccsSynced cache.InformerSynced
	logger        log.Logger

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
	return nil
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

// deleteDistccClient takes a Distcc resource and converts it into a
// namespace/name string which is then put onto the work queue. This method
// should *not* be passed resources of any type other than DistccClient.
func (c *controller) deleteDistccClient(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	// the object is gone: delete it from the queue
	c.workqueue.Forget(key)
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
