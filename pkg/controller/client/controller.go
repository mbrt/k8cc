package client

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	sharedctr "github.com/mbrt/k8cc/pkg/controller"
	"github.com/mbrt/k8cc/pkg/conv"
	k8ccerr "github.com/mbrt/k8cc/pkg/errors"
)

const controllerAgentName = "k8cc-distcc-client-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Distcc is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Distcc fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"
	// ErrAmbiguousLabels is used as part of the Event 'reason' when a Distcc fails
	// to sync due to multiple deployment satisfying the label selector.
	ErrAmbiguousLabels = "ErrAmbiguousMatch"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by DistccClientClaim"
	// MessageResourceSynced is the message used for an Event fired when a Distcc
	// is synced successfully
	MessageResourceSynced = "DistccClientClaim synced successfully"

	// UserLabel is the label used to identify a resource given to a particular user
	UserLabel = "k8cc.io/username"

	// SSHPort is the port used by the ssh daemon
	SSHPort = 22
)

// Controller is responsible for DistccClient and DistccClientClaim objects management
type Controller interface {
	Run(threadiness int, stopCh <-chan struct{}) error
}

// NewController creates a new Controller
func NewController(
	sharedClient *sharedctr.SharedClient,
	logger log.Logger,
) Controller {
	deployInformer := sharedClient.KubeInformerFactory.Apps().V1beta2().Deployments()
	serviceInformer := sharedClient.KubeInformerFactory.Core().V1().Services()
	distccInformer := sharedClient.DistccInformerFactory.K8cc().V1alpha1().DistccClients()
	claimInformer := sharedClient.DistccInformerFactory.K8cc().V1alpha1().DistccClientClaims()

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
		kubeclientset:      sharedClient.KubeClientset,
		k8ccclientset:      sharedClient.K8ccClientset,
		deploymentsLister:  deployInformer.Lister(),
		deploymentsSynced:  deployInformer.Informer().HasSynced,
		servicesLister:     serviceInformer.Lister(),
		servicesSynced:     serviceInformer.Informer().HasSynced,
		distccsLister:      distccInformer.Lister(),
		distccsSynced:      distccInformer.Informer().HasSynced,
		distccclaimsLister: claimInformer.Lister(),
		distccclaimsSynced: claimInformer.Informer().HasSynced,
		logger:             logger,
		workqueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "k8cc-distcc-client"),
		recorder:           recorder,
	}

	// Set up an event handler for when Claim resources change
	claimInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: op.enqueueClientClaim,
		UpdateFunc: func(old, new interface{}) {
			// we ignore if old == new. we take advantage of periodic
			// updates to manage downscaling periodically
			op.enqueueClientClaim(new)
		},
		DeleteFunc: op.enqueueClientClaim,
	})

	// TODO: handle client by enqueuing all the referencing claims
	distccInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: op.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newClient := new.(*k8ccv1alpha1.DistccClient)
			oldClient := old.(*k8ccv1alpha1.DistccClient)
			if newClient.ResourceVersion == oldClient.ResourceVersion {
				// Periodic resync will send update events for all known DistccClients.
				// Two different versions of the same resource will always have different RVs.
				return
			}
			op.handleObject(new)
		},
		DeleteFunc: op.handleObject,
	})

	deployInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: op.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDeploy := new.(*appsv1beta2.Deployment)
			oldDeploy := old.(*appsv1beta2.Deployment)
			if newDeploy.ResourceVersion == oldDeploy.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same resource will always have different RVs.
				return
			}
			op.handleObject(new)
		},
		DeleteFunc: op.handleObject,
	})

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: op.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newService := new.(*corev1.Service)
			oldService := old.(*corev1.Service)
			if newService.ResourceVersion == oldService.ResourceVersion {
				// Periodic resync will send update events for all known Services.
				// Two different versions of the same resource will always have different RVs.
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
	deploymentsLister  appslisters.DeploymentLister
	deploymentsSynced  cache.InformerSynced
	servicesLister     corelisters.ServiceLister
	servicesSynced     cache.InformerSynced
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

	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.servicesSynced, c.distccsSynced, c.distccclaimsSynced); !ok {
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
	changed := componentsChange{}
	if changed.Expiration = c.updateExpiration(claim, dclient); changed.Expiration {
		_ = c.logger.Log("method", "syncHandler", "info",
			fmt.Sprintf("updated expiration to %s", claim.Status.ExpirationTime))
	}

	// Check expiration time, and in case delete the resource
	if c.isExpired(claim, dclient) {
		return c.k8ccclientset.K8ccV1alpha1().DistccClientClaims(namespace).Delete(name, nil)
	}

	// Check the related Deployment
	changed.Deploy, err = c.syncDeploy(claim, dclient)
	if err != nil {
		// If an error is transient, we'll requeue the item so we can attempt
		// processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		// Otherwise we just fail permanently
		if k8ccerr.IsTransient(err) {
			return err
		}
		runtime.HandleError(err)
		return nil
	}

	// Check the related Service
	changed.Service, err = c.syncService(claim, dclient)
	if err != nil {
		if k8ccerr.IsTransient(err) {
			return err
		}
		runtime.HandleError(err)
		return nil
	}

	if changed.Any() {
		_, err = c.k8ccclientset.K8ccV1alpha1().DistccClientClaims(namespace).Update(claim)
	}

	return err
}

func (c *controller) syncDeploy(claim *k8ccv1alpha1.DistccClientClaim, client *k8ccv1alpha1.DistccClient) (bool, error) {
	// Try to retreive the existing deployment through the status reference, or the labels selector
	obj, err := sharedctr.GetObjectFromReferenceOrSelector(
		sharedctr.NewDeploymentLister(c.deploymentsLister),
		claim.Namespace, claim.Status.Deployment,
		labels.Set(getLabelsForUser(client.Labels, claim.Spec.UserName)).AsSelector(),
	)
	if err != nil {
		if sharedctr.IsAmbiguousReference(err) {
			msg := fmt.Sprintf("Deployment ref resolution failed: %s", err.Error())
			c.recorder.Event(claim, corev1.EventTypeWarning, ErrAmbiguousLabels, msg)
			return false, k8ccerr.TransientError(errors.Errorf(msg))
		}
		return false, err
	}

	var deploy *appsv1beta2.Deployment
	if obj != nil {
		deploy = obj.(*appsv1beta2.Deployment)
	} else {
		// No way to get a deployment: need to create a new one
		var err error
		new := newDeploy(claim, client)
		deploy, err = c.kubeclientset.AppsV1beta2().Deployments(claim.Namespace).Create(new)
		// If an error occurs during Get/Create, we'll requeue the item so we can
		// attempt processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		if err != nil {
			return false, k8ccerr.TransientError(err)
		}
	}

	// If the Deployment is not controlled by this Distcc resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(deploy, claim) {
		msg := fmt.Sprintf(MessageResourceExists, deploy.Name)
		c.recorder.Event(claim, corev1.EventTypeWarning, ErrResourceExists, msg)
		return false, k8ccerr.TransientError(errors.Errorf(msg))
	}

	// Try to update the state of the claim, with the link to the deployment, if it was not correct
	updated := false
	if deploy.Name != "" {
		if claim.Status.Deployment == nil || claim.Status.Deployment.Name != deploy.Name {
			claim.Status.Deployment = &corev1.LocalObjectReference{Name: deploy.Name}
			updated = true
		}
	} else {
		if claim.Status.Deployment != nil {
			claim.Status.Deployment = nil
			updated = true
		}
	}

	return updated, nil
}

func (c *controller) syncService(claim *k8ccv1alpha1.DistccClientClaim, client *k8ccv1alpha1.DistccClient) (bool, error) {
	// Try to retreive the existing deployment through the status reference, or the labels selector
	obj, err := sharedctr.GetObjectFromReferenceOrSelector(
		sharedctr.NewServiceLister(c.servicesLister),
		claim.Namespace, claim.Status.Service,
		labels.Set(getLabelsForUser(client.Labels, claim.Spec.UserName)).AsSelector(),
	)
	if err != nil {
		if sharedctr.IsAmbiguousReference(err) {
			msg := fmt.Sprintf("Service ref resolution failed: %s", err.Error())
			c.recorder.Event(claim, corev1.EventTypeWarning, ErrAmbiguousLabels, msg)
			return false, k8ccerr.TransientError(errors.Errorf(msg))
		}
		return false, err
	}

	var service *corev1.Service
	if obj != nil {
		service = obj.(*corev1.Service)
	} else {
		// No way to get a service: need to create a new one
		var err error
		new := newService(claim, client)
		service, err = c.kubeclientset.CoreV1().Services(claim.Namespace).Create(new)
		// If an error occurs during Get/Create, we'll requeue the item so we can
		// attempt processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		if err != nil {
			return false, k8ccerr.TransientError(err)
		}
	}

	// If the Service is not controlled by this Distcc resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(service, claim) {
		msg := fmt.Sprintf(MessageResourceExists, service.Name)
		c.recorder.Event(claim, corev1.EventTypeWarning, ErrResourceExists, msg)
		return false, k8ccerr.TransientError(errors.Errorf(msg))
	}

	// Try to update the state of the claim, with the link to the deployment, if it was not correct
	updated := false
	if service.Name != "" {
		if claim.Status.Service == nil || claim.Status.Service.Name != service.Name {
			claim.Status.Service = &corev1.LocalObjectReference{Name: service.Name}
			updated = true
		}
	} else {
		if claim.Status.Service != nil {
			claim.Status.Service = nil
			updated = true
		}
	}

	return updated, nil
}

// enqueueClientClaim takes a DistccClientClaim resource and converts it into a
// namespace/name string which is then put onto the work queue. This method
// should *not* be passed resources of any type other than DistccClient.
func (c *controller) enqueueClientClaim(obj interface{}) {
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
		if ownerRef.Kind != "DistccClientClaim" {
			return
		}

		dclaim, err := c.distccclaimsLister.DistccClientClaims(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			_ = c.logger.Log("method", "handleObject", "err", err.Error(), "info",
				fmt.Sprintf("ignore orphaned %s with owner %s", object.GetSelfLink(), ownerRef.Name))
			return
		}

		c.enqueueClientClaim(dclaim)
		return
	}
}

func (c *controller) updateExpiration(claim *k8ccv1alpha1.DistccClientClaim, client *k8ccv1alpha1.DistccClient) bool {
	now := time.Now()
	maxExpiration := now.Add(client.Spec.LeaseDuration.Duration)
	// If expiration is not set, set it automatically
	// If the expiration exceedes the maximum possible one (namely now + expiration time)
	// then reduce it to that value
	if claim.Status.ExpirationTime == nil || claim.Status.ExpirationTime.After(maxExpiration) {
		claim.Status.ExpirationTime = conv.ToKubeTime(maxExpiration)
		return true
	}
	return false
}

func (c *controller) isExpired(claim *k8ccv1alpha1.DistccClientClaim, client *k8ccv1alpha1.DistccClient) bool {
	return time.Now().After(claim.Status.ExpirationTime.Time)
}

func newDeploy(claim *k8ccv1alpha1.DistccClientClaim, client *k8ccv1alpha1.DistccClient) *appsv1beta2.Deployment {
	// always one POD per client
	var replicas int32 = 1
	labels := getLabelsForUser(client.Labels, claim.Spec.UserName)
	// update the template with
	// - the required secrets
	// - the labels coming from the claim
	template := client.Spec.Template.DeepCopy()
	for _, secret := range claim.Spec.Secrets {
		volume := corev1.Volume{
			Name: secret.Name,
			VolumeSource: corev1.VolumeSource{
				Secret: &secret.VolumeSource,
			},
		}
		template.Spec.Volumes = append(template.Spec.Volumes, volume)
	}
	template.Labels = labels

	return &appsv1beta2.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", claim.Name),
			Namespace:    claim.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(claim, schema.GroupVersionKind{
					Group:   k8ccv1alpha1.SchemeGroupVersion.Group,
					Version: k8ccv1alpha1.SchemeGroupVersion.Version,
					Kind:    "DistccClientClaim",
				}),
			},
			Labels: labels,
		},
		Spec: appsv1beta2.DeploymentSpec{
			Replicas: &replicas,
			Selector: getSelectorForUser(client.Spec.Selector, claim.Spec.UserName),
			Template: *template,
			Strategy: appsv1beta2.DeploymentStrategy{
				Type: appsv1beta2.RollingUpdateDeploymentStrategyType,
			},
		},
	}
}

func newService(claim *k8ccv1alpha1.DistccClientClaim, client *k8ccv1alpha1.DistccClient) *corev1.Service {
	selector := getLabelsForUser(client.Labels, claim.Spec.UserName)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", claim.Name),
			Namespace:    claim.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(claim, schema.GroupVersionKind{
					Group:   k8ccv1alpha1.SchemeGroupVersion.Group,
					Version: k8ccv1alpha1.SchemeGroupVersion.Version,
					Kind:    "DistccClientClaim",
				}),
			},
			Labels: selector,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Port: SSHPort},
			},
			Selector: selector,
			Type:     corev1.ServiceTypeNodePort,
		},
	}
}

func getLabelsForUser(labels map[string]string, user string) map[string]string {
	// deep copy + update the user label
	res := map[string]string{}
	for k, v := range labels {
		res[k] = v
	}
	res[UserLabel] = user
	return res
}

func getSelectorForUser(selector *metav1.LabelSelector, user string) *metav1.LabelSelector {
	if selector == nil || selector.MatchLabels == nil {
		return nil
	}
	return &metav1.LabelSelector{
		MatchLabels:      getLabelsForUser(selector.MatchLabels, user),
		MatchExpressions: selector.MatchExpressions,
	}
}

type componentsChange struct {
	Deploy     bool
	Expiration bool
	Service    bool
}

func (d componentsChange) Any() bool {
	return d.Deploy ||
		d.Expiration ||
		d.Service
}
