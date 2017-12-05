package kube

// see https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
// and https://github.com/kubernetes/sample-controller/blob/master/controller.go
// and https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/podautoscaler/horizontal.go

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1beta2"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"

	"github.com/mbrt/k8cc/pkg/data"
)

// operator controls tag deployments as a regular Kubernetes operator
type operator struct {
	kubeclientset        kubernetes.Interface
	statefulsetLister    appslisters.StatefulSetLister
	statefulsetSynced    cache.InformerSynced
	desiredReplicasCache DesiredReplicasCache

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
}

// NewOperator creates an Operator using default the given settings for the kube connection.
//
// If kubecfg and masterURL are empty, defaults to in-cluster configuration
func NewOperator(masterURL, kubecfg string, drc DesiredReplicasCache) (Operator, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubecfg)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	return newOperator(kubeClient, kubeInformerFactory, drc), nil
}

func newOperator(
	kubeclientset kubernetes.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	desiredReplicasCache DesiredReplicasCache,
) Operator {
	statefulsetInformer := kubeInformerFactory.Apps().V1beta2().StatefulSets()

	op := operator{
		kubeclientset:        kubeclientset,
		statefulsetLister:    statefulsetInformer.Lister(),
		statefulsetSynced:    statefulsetInformer.Informer().HasSynced,
		desiredReplicasCache: desiredReplicasCache,
		workqueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "k8cc-stateful-sets"),
	}

	// Set up an event handler for when StatefulSet resources change. This
	// way, we don't need to implement custom logic for handling StatefulSet
	// resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	statefulsetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: op.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newSet := new.(*appsv1beta2.StatefulSet)
			oldSet := old.(*appsv1beta2.StatefulSet)
			if newSet.ResourceVersion == oldSet.ResourceVersion {
				// Periodic resync will send update events for all known StatefulSets.
				// Two different versions of the same StatefulSet will always have different RVs.
				return
			}
			op.handleObject(new)
		},
		DeleteFunc: op.handleObject,
	})

	return &op
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *operator) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	if ok := cache.WaitForCacheSync(stopCh, c.statefulsetSynced); !ok {
		return errors.New("failed to wait for caches to sync")
	}

	// Launch two workers to process StatefulSet resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	return nil
}

func (c *operator) NotifyUpdated(t data.Tag) error {
	ls := labels.Set{BuildTagLabel: string(t)}
	sets, err := c.statefulsetLister.List(ls.AsSelector())
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("error listing the stateful set related to tag %s", t))
	}
	for _, s := range sets {
		c.enqueueStatefulSet(s)
	}
	return nil
}

func (c *operator) Hostnames(t data.Tag, ids []data.HostID) ([]string, error) {
	ls := labels.Set{BuildTagLabel: string(t)}
	sets, err := c.statefulsetLister.List(ls.AsSelector())
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("error listing the stateful set related to tag %s", t))
	}
	if len(sets) == 0 {
		return nil, errors.New("no related stateful set has been found")
	}

	// TODO which namespace??
	set := sets[0]
	pn := set.Spec.Template.Name
	sn := set.Spec.ServiceName
	r := make([]string, len(ids))
	for i, id := range ids {
		r[i] = fmt.Sprintf("%s-%d.%s", pn, id, sn)
	}
	return r, nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *operator) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *operator) processNextWorkItem() bool {
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
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
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
func (c *operator) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the stateful set resource with this namespace/name
	statefulset, err := c.statefulsetLister.StatefulSets(namespace).Get(name)
	if err != nil {
		// The Foo resource may no longer exist, in which case we stop
		// processing.
		if kubeerr.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("statefulset '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	// get the tag from the labels
	var tag string
	var ok bool
	if tag, ok = statefulset.Labels[StatefulSetLabel]; !ok {
		runtime.HandleError(fmt.Errorf("statefulset '%s' in work queue doesn't have required label", key))
		return nil
	}

	desiredReplicas := c.desiredReplicasCache.DesiredReplicas(data.Tag(tag))
	if desiredReplicas == statefulset.Status.Replicas {
		// nothing to update
		return nil

	}

	// copy the object, to avoid changing the shared cached one
	// update the replicas
	statefulset = statefulset.DeepCopy()
	statefulset.Spec.Replicas = &desiredReplicas
	_, err = c.kubeclientset.AppsV1beta2().StatefulSets(statefulset.Namespace).Update(statefulset)
	return err
}

// enqueueStatefulSet takes a StatefulSet resource and converts it into a
// namespace/name string which is then put onto the work queue. This method
// should *not* be passed resources of any type other than StatefulSet.
func (c *operator) enqueueStatefulSet(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Foo resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Foo resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *operator) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(errors.New("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(errors.New("error decoding object tombstone, invalid type"))
			return
		}
	}
	if tag, ok := object.GetLabels()[StatefulSetLabel]; ok && tag == StatefulSetVersion {
		// this object is controlled by us, enqueue it
		c.enqueueStatefulSet(object)
	}
}

// DesiredReplicasCache provides the numebr of desired replicas for a certain tag
type DesiredReplicasCache interface {
	// DesiredReplicas returns the number of desired replicas for the
	// stateful set associated with a tag
	DesiredReplicas(t data.Tag) int32
}
