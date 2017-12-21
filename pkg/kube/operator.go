package kube

// see https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
// and https://github.com/kubernetes/sample-controller/blob/master/controller.go
// and https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/podautoscaler/horizontal.go

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1beta2"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	k8ccv1alpha1 "github.com/mbrt/k8cc/pkg/apis/k8cc.io/v1alpha1"
	clientset "github.com/mbrt/k8cc/pkg/client/clientset/versioned"
	k8ccscheme "github.com/mbrt/k8cc/pkg/client/clientset/versioned/scheme"
	listers "github.com/mbrt/k8cc/pkg/client/listers/k8cc/v1alpha1"
	"github.com/mbrt/k8cc/pkg/data"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Distcc is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Distcc fails
	// to sync due to a StatefulSet of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a StatefulSet already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Distcc"
	// MessageResourceSynced is the message used for an Event fired when a Distcc
	// is synced successfully
	MessageResourceSynced = "Distcc synced successfully"

	// DistccPort is the port used by distcc daemons.
	DistccPort = 3632
)

const controllerAgentName = "k8cc-controller"

// operator controls tag deployments as a regular Kubernetes operator
type operator struct {
	kubeclientset     kubernetes.Interface
	k8ccclientset     clientset.Interface
	statefulsetLister appslisters.StatefulSetLister
	statefulsetSynced cache.InformerSynced
	serviceLister     corelisters.ServiceLister
	serviceSynced     cache.InformerSynced
	distccsLister     listers.DistccLister
	distccsSynced     cache.InformerSynced
	desiredState      DesiredStateProvider
	logger            log.Logger

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

// NewOperator creates an Operator using the given shared client connection.
func NewOperator(
	sharedClient *SharedClient,
	desiredState DesiredStateProvider,
	logger log.Logger,
) Operator {
	statefulsetInformer := sharedClient.kubeInformerFactory.Apps().V1beta2().StatefulSets()
	serviceInformer := sharedClient.kubeInformerFactory.Core().V1().Services()
	distccInformer := sharedClient.distccInformerFactory.K8cc().V1alpha1().Distccs()

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	k8ccscheme.AddToScheme(scheme.Scheme)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(func(format string, args ...interface{}) {
		_ = logger.Log("event", fmt.Sprintf(format, args...))
	})
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: sharedClient.kubeclientset.CoreV1().Events("")})

	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	op := operator{
		kubeclientset:     sharedClient.kubeclientset,
		k8ccclientset:     sharedClient.k8ccclientset,
		statefulsetLister: statefulsetInformer.Lister(),
		statefulsetSynced: statefulsetInformer.Informer().HasSynced,
		serviceLister:     serviceInformer.Lister(),
		serviceSynced:     serviceInformer.Informer().HasSynced,
		distccsLister:     distccInformer.Lister(),
		distccsSynced:     distccInformer.Informer().HasSynced,
		desiredState:      desiredState,
		logger:            logger,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "k8cc-stateful-sets"),
		recorder:          recorder,
	}

	// Set up an event handler for when Distcc resources change
	distccInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: op.enqueueDistcc,
		UpdateFunc: func(old, new interface{}) {
			// we ignore if old == new. we take advantage of periodic
			// updates to manage downscaling periodically
			op.enqueueDistcc(new)
		},
		DeleteFunc: op.deleteDistcc,
	})

	// Set up an event handler for when StatefulSet resources change. This
	// way, we don't need to implement custom logic for handling StatefulSet
	// resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	statefulsetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: op.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newSS := new.(*appsv1beta2.StatefulSet)
			oldSS := old.(*appsv1beta2.StatefulSet)
			if newSS.ResourceVersion == oldSS.ResourceVersion {
				// Periodic resync will send update events for all known StatefulSets.
				// Two different versions of the same StatefulSet will always have different RVs.
				return
			}
			op.handleObject(new)
		},
		DeleteFunc: op.handleObject,
	})

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: op.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newS := new.(*corev1.Service)
			oldS := old.(*corev1.Service)
			if newS.ResourceVersion == oldS.ResourceVersion {
				// Periodic resync will send update events for all known Services.
				// Two different versions of the same Service will always have different RVs.
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

	if ok := cache.WaitForCacheSync(stopCh, c.statefulsetSynced, c.distccsSynced); !ok {
		return errors.New("failed to wait for caches to sync")
	}

	// Launch workers to process StatefulSet resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	return nil
}

func (c *operator) NotifyUpdated(t data.Tag) error {
	_ = c.logger.Log("method", "notifyUpdated", "tag", t)
	distcc, err := c.distccsLister.Distccs(t.Namespace).Get(t.Name)
	if err != nil {
		return err
	}
	c.enqueueDistcc(distcc)
	return nil
}

func (c *operator) Hostnames(t data.Tag, ids []data.HostID) ([]string, error) {
	distcc, err := c.distccsLister.Distccs(t.Namespace).Get(t.Name)
	if err != nil {
		return nil, err
	}
	r := make([]string, len(ids))
	for i, id := range ids {
		r[i] = fmt.Sprintf("%s-%d.%s", distcc.Spec.DeploymentName, id, distcc.Spec.ServiceName)
	}
	return r, nil
}

func (c *operator) ScaleSettings(t data.Tag) (data.ScaleSettings, error) {
	distcc, err := c.distccsLister.Distccs(t.Namespace).Get(t.Name)
	if err != nil {
		return data.ScaleSettings{}, err
	}
	minReplicas := 0
	if distcc.Spec.MinReplicas != nil {
		minReplicas = int(*distcc.Spec.MinReplicas)
	}
	return data.ScaleSettings{
		MinReplicas:     minReplicas,
		MaxReplicas:     int(distcc.Spec.MaxReplicas),
		ReplicasPerUser: int(distcc.Spec.UserReplicas),
		LeaseTime:       distcc.Spec.LeaseDuration.Duration,
	}, nil
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
func (c *operator) syncHandler(key string) error {
	_ = c.logger.Log("method", "syncHandler", "key", key)

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(errors.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Distcc resource with this namespace/name
	distcc, err := c.distccsLister.Distccs(namespace).Get(name)
	if err != nil {
		// The Distcc resource may no longer exist, in which case we stop
		// processing.
		if kubeerr.IsNotFound(err) {
			runtime.HandleError(errors.Errorf("distcc '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	update, err := c.syncStatefulSet(distcc)
	if err != nil {
		// If an error is transient, we'll requeue the item so we can attempt
		// processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		// Otherwise we just fail permanently
		if IsTransient(err) {
			return err
		}
		runtime.HandleError(err)
		return nil
	}

	update.ServiceCreated, err = c.syncService(distcc)
	if err != nil {
		if IsTransient(err) {
			return err
		}
		runtime.HandleError(err)
		return nil
	}

	// Finally, we update the status block of the Distcc resource to reflect the
	// current state of the world
	err = c.updateDistccStatus(distcc, update)
	if err != nil {
		return err
	}

	if update.Any() {
		// Fire a sync event only if there's been an update
		c.recorder.Event(distcc, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	}
	return nil
}

func (c *operator) syncStatefulSet(distcc *k8ccv1alpha1.Distcc) (distccUpdateState, error) {
	// The build tag is the name of the Distcc resource
	tag := data.Tag{Namespace: distcc.Namespace, Name: distcc.Name}
	var state distccUpdateState

	statefulName := distcc.Spec.DeploymentName
	if statefulName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		return state, errors.Errorf("%s: deployment name must be specified", tag)
	}

	// Get the statefulset with the name specified in Distcc.Spec
	stateful, err := c.statefulsetLister.StatefulSets(distcc.Namespace).Get(statefulName)
	// If the resource doesn't exist, we'll create it
	if kubeerr.IsNotFound(err) {
		new := newStatefulSet(distcc, nil)
		stateful, err = c.kubeclientset.AppsV1beta2().StatefulSets(distcc.Namespace).Create(new)
		state.StatefulCreated = true
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return state, transientError{err}
	}

	// If the StatefulSet is not controlled by this Distcc resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(stateful, distcc) {
		msg := fmt.Sprintf(MessageResourceExists, stateful.Name)
		c.recorder.Event(distcc, corev1.EventTypeWarning, ErrResourceExists, msg)
		return state, transientError{errors.Errorf(msg)}
	}

	// Determine the desired replicas
	desiredReplicas := c.desiredState.Replicas(tag)
	// Update the number of replicas, in case it doesn't match the desired
	if needScale(distcc, stateful, desiredReplicas) {
		new := newStatefulSet(distcc, &desiredReplicas)
		_, err = c.kubeclientset.AppsV1beta2().StatefulSets(distcc.Namespace).Update(new)

		// If an error occurs during Update, we'll requeue the item so we can
		// attempt processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		if err != nil {
			return state, transientError{err}
		}
		state.StatefulScaled = true
	}

	return state, nil
}

func (c *operator) syncService(distcc *k8ccv1alpha1.Distcc) (bool, error) {
	// The build tag is the name of the Distcc resource
	tag := data.Tag{Namespace: distcc.Namespace, Name: distcc.Name}

	serviceName := distcc.Spec.ServiceName
	if serviceName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		return false, errors.Errorf("%s: service name must be specified", tag)
	}

	if distcc.Spec.Selector == nil || len(distcc.Spec.Selector.MatchExpressions) > 0 {
		return false, errors.Errorf("%s: selector must present, and be a 'matchLabels'", tag)
	}

	// Get the service with the name specified in Distcc.Spec
	updated := false
	service, err := c.serviceLister.Services(distcc.Namespace).Get(serviceName)
	// If the resource doesn't exist, we'll create it
	if kubeerr.IsNotFound(err) {
		new := newService(distcc)
		service, err = c.kubeclientset.CoreV1().Services(distcc.Namespace).Create(new)
		updated = true
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return updated, transientError{err}
	}

	// If the Service is not controlled by this Distcc resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(service, distcc) {
		msg := fmt.Sprintf(MessageResourceExists, service.Name)
		c.recorder.Event(distcc, corev1.EventTypeWarning, ErrResourceExists, msg)
		return updated, transientError{errors.Errorf(msg)}
	}

	return updated, nil
}

func (c *operator) updateDistccStatus(distcc *k8ccv1alpha1.Distcc, updated distccUpdateState) error {
	// Get the leases from the desired state and serialize it into the
	// DistccState version. If the state hasn't changed, then don't
	// update the distcc object
	tag := data.Tag{Namespace: distcc.Namespace, Name: distcc.Name}
	leases := c.desiredState.Leases(tag)
	leasesState := make([]k8ccv1alpha1.DistccLease, len(leases))
	for i, lease := range leases {
		hosts := make([]int32, len(lease.Hosts))
		for i, h := range lease.Hosts {
			hosts[i] = int32(h)
		}
		leasesState[i] = k8ccv1alpha1.DistccLease{
			UserName:       string(lease.User),
			ExpirationTime: *toKubeTime(lease.Expiration),
			AssignedHosts:  hosts,
		}
	}
	if !updated.Any() && isStateEqual(leasesState, distcc.Status.Leases) {
		return nil
	}

	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	distccCopy := distcc.DeepCopy()
	now := toKubeTime(time.Now())
	distccCopy.Status.LastUpdateTime = now
	if updated.StatefulScaled {
		distccCopy.Status.LastScaleTime = now
	}
	distccCopy.Status.Leases = leasesState
	_, err := c.k8ccclientset.K8ccV1alpha1().Distccs(distcc.Namespace).Update(distccCopy)
	return err
}

// enqueueDistcc takes a StatefulSet resource and converts it into a
// namespace/name string which is then put onto the work queue. This method
// should *not* be passed resources of any type other than Distcc.
func (c *operator) enqueueDistcc(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// enqueueDistcc takes a Distcc resource and converts it into a
// namespace/name string which is then put onto the work queue. This method
// should *not* be passed resources of any type other than Distcc.
func (c *operator) deleteDistcc(obj interface{}) {
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
// to find the Distcc resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Distcc resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *operator) handleObject(obj interface{}) {
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
		if ownerRef.Kind != "Distcc" {
			return
		}

		distcc, err := c.distccsLister.Distccs(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			_ = c.logger.Log("method", "handleObject", "err", err.Error(), "info",
				fmt.Sprintf("ignore orphaned %s with owner %s", object.GetSelfLink(), ownerRef.Name))
			return
		}

		c.enqueueDistcc(distcc)
		return
	}
}

// newStatefulSet creates a new StatefulSet for a Distcc resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the StatefulSet resource that 'owns' it. The number of replicas is optional.
func newStatefulSet(distcc *k8ccv1alpha1.Distcc, replicas *int32) *appsv1beta2.StatefulSet {
	return &appsv1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      distcc.Spec.DeploymentName,
			Namespace: distcc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(distcc, schema.GroupVersionKind{
					Group:   k8ccv1alpha1.SchemeGroupVersion.Group,
					Version: k8ccv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Distcc",
				}),
			},
			Labels: distcc.Labels,
		},
		Spec: appsv1beta2.StatefulSetSpec{
			Replicas:            replicas,
			Selector:            distcc.Spec.Selector,
			Template:            distcc.Spec.Template,
			ServiceName:         distcc.Spec.ServiceName,
			PodManagementPolicy: appsv1beta2.ParallelPodManagement,
			UpdateStrategy: appsv1beta2.StatefulSetUpdateStrategy{
				Type: appsv1beta2.RollingUpdateStatefulSetStrategyType,
			},
		},
	}
}

func newService(distcc *k8ccv1alpha1.Distcc) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      distcc.Spec.ServiceName,
			Namespace: distcc.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(distcc, schema.GroupVersionKind{
					Group:   k8ccv1alpha1.SchemeGroupVersion.Group,
					Version: k8ccv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Distcc",
				}),
			},
			Labels: distcc.Labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: DistccPort},
			},
			Selector:  distcc.Spec.Selector.MatchLabels,
			ClusterIP: "None",
		},
	}
}

func needScale(distcc *k8ccv1alpha1.Distcc, stateful *appsv1beta2.StatefulSet, desiredReplicas int32) bool {
	if stateful.Spec.Replicas == nil {
		// The replicas are not set
		return true
	}
	currentReplicas := *stateful.Spec.Replicas

	switch {
	case currentReplicas < desiredReplicas:
		// Upscale always allowed
		return true
	case currentReplicas == desiredReplicas:
		// Not needed
		return false
	}

	// Downscale: check if the downscale window is respected
	if distcc.Spec.DownscaleWindow == nil || distcc.Status.LastScaleTime == nil {
		return true
	}
	nextAllowedTime := distcc.Status.LastScaleTime.Add(distcc.Spec.DownscaleWindow.Duration)
	return time.Now().After(nextAllowedTime)
}

func toKubeTime(t time.Time) *metav1.Time {
	// It's necessary to truncate nanoseconds to allow correct comparison
	r := metav1.NewTime(t).Rfc3339Copy()
	return &r
}

func isStateEqual(a, b []k8ccv1alpha1.DistccLease) bool {
	// Unfortunately DeepEqual doesn't work with time,
	// so I needed to rollout my own Equal
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		la := a[i]
		lb := b[i]
		if la.UserName != lb.UserName ||
			!la.ExpirationTime.Equal(&lb.ExpirationTime) ||
			len(la.AssignedHosts) != len(lb.AssignedHosts) {
			return false
		}
		for i := range la.AssignedHosts {
			if la.AssignedHosts[i] != lb.AssignedHosts[i] {
				return false
			}
		}
	}
	return true
}

type distccUpdateState struct {
	StatefulCreated bool
	StatefulScaled  bool
	ServiceCreated  bool
}

func (d distccUpdateState) Any() bool {
	return d.StatefulCreated ||
		d.StatefulScaled ||
		d.ServiceCreated
}

// DesiredStateProvider provides the numebr of desired replicas for a certain tag
type DesiredStateProvider interface {
	// Replicas returns the number of desired replicas for the stateful set
	// associated with a tag
	Replicas(t data.Tag) int32
	// Leases returns all the active leases
	Leases(t data.Tag) []data.Lease
}

// IsTransient returns true if an error is transient
func IsTransient(err error) bool {
	te, ok := errors.Cause(err).(transient)
	return ok && te.Transient()
}

type transient interface {
	Transient() bool
}

type transientError struct {
	error
}

func (e transientError) Error() string   { return e.error.Error() }
func (e transientError) Transient() bool { return true }
