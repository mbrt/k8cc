package distcc

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1beta2"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	k8ccv1alpha1 "github.com/mbrt/k8cc/pkg/apis/k8cc.io/v1alpha1"
	clientset "github.com/mbrt/k8cc/pkg/client/clientset/versioned"
	listers "github.com/mbrt/k8cc/pkg/client/listers/k8cc.io/v1alpha1"
	sharedctr "github.com/mbrt/k8cc/pkg/controller"
	"github.com/mbrt/k8cc/pkg/controller/kit"
	"github.com/mbrt/k8cc/pkg/conv"
	k8ccerr "github.com/mbrt/k8cc/pkg/errors"
)

const (
	// ErrResourceExists is used as part of the Event 'reason' when a Distcc fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Distcc"

	// DistccPort is the port used by distcc daemons.
	DistccPort = 3632
)

const controllerAgentName = "k8cc-distcc-controller"

// controller controls Distcc objects
type controller struct {
	kubeclientset kubernetes.Interface
	k8ccclientset clientset.Interface
	deployLister  appslisters.DeploymentLister
	serviceLister corelisters.ServiceLister
	distccsLister listers.DistccLister
	claimsLister  listers.DistccClaimLister
}

// NewController creates a controller for Distcc using the given shared client connection.
func NewController(sharedClient *sharedctr.SharedClient) sharedctr.Controller {
	deployInformer := sharedClient.KubeInformerFactory.Apps().V1beta2().Deployments()
	serviceInformer := sharedClient.KubeInformerFactory.Core().V1().Services()
	distccInformer := sharedClient.DistccInformerFactory.K8cc().V1alpha1().Distccs()
	claimInformer := sharedClient.DistccInformerFactory.K8cc().V1alpha1().DistccClaims()

	handler := controller{
		kubeclientset: sharedClient.KubeClientset,
		k8ccclientset: sharedClient.K8ccClientset,
		deployLister:  deployInformer.Lister(),
		serviceLister: serviceInformer.Lister(),
		distccsLister: distccInformer.Lister(),
		claimsLister:  claimInformer.Lister(),
	}

	return kit.NewController(
		&handler,
		sharedClient.KubeClientset,
		distccInformer.Informer(),
		[]cache.SharedInformer{
			deployInformer.Informer(),
			serviceInformer.Informer(),
			claimInformer.Informer(),
		},
	)
}

func (c *controller) AgentName() string {
	return controllerAgentName
}

func (c *controller) Sync(object runtime.Object) (bool, error) {
	distcc := object.(*k8ccv1alpha1.Distcc)

	update, err := c.syncDeployment(distcc)
	if err != nil {
		return update.Any(), err
	}

	update.ServiceCreated, err = c.syncService(distcc)
	if err != nil {
		return update.Any(), err
	}

	// Finally, we update the status block of the Distcc resource to reflect the
	// current state of the world
	err = c.updateDistccStatus(distcc, update)
	if err != nil {
		return update.Any(), err
	}

	return update.Any(), nil
}

func (c *controller) CustomResourceKind() string {
	return "Distcc"
}

func (c *controller) CustomResourceInstance(namespace, name string) (runtime.Object, error) {
	return c.distccsLister.Distccs(namespace).Get(name)
}

func (c *controller) NeedPeriodicSync() bool {
	return true
}

func (c *controller) OnControlledObjectUpdate(object interface{}) (interface{}, error) {
	// We want to enqueue the Distcc referenced by a DistccClaim
	claim, ok := object.(*k8ccv1alpha1.DistccClaim)
	if !ok {
		// Not interested in this object
		return nil, nil
	}
	return c.distccsLister.Distccs(claim.Namespace).Get(claim.Spec.DistccName)
}

func (c *controller) syncDeployment(distcc *k8ccv1alpha1.Distcc) (distccUpdateState, error) {
	var state distccUpdateState

	deployName := distcc.Spec.DeploymentName
	if deployName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		return state, errors.Errorf("%s: deployment name must be specified", deployName)
	}

	// Get the deploy with the name specified in Distcc.Spec
	deploy, err := c.deployLister.Deployments(distcc.Namespace).Get(deployName)
	// If the resource doesn't exist, we'll create it
	if kubeerr.IsNotFound(err) {
		new := newDeployment(distcc, nil)
		deploy, err = c.kubeclientset.AppsV1beta2().Deployments(distcc.Namespace).Create(new)
		state.StatefulCreated = true
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return state, k8ccerr.TransientError(err)
	}

	// If the Deployment is not controlled by this Distcc resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(deploy, distcc) {
		msg := fmt.Sprintf(MessageResourceExists, deploy.Name)
		return state, kit.EventfulError(errors.New(msg), kit.Event{
			Type:    corev1.EventTypeWarning,
			Reason:  ErrResourceExists,
			Message: msg,
		})
	}

	// Determine the desired replicas
	desiredReplicas, err := c.desiredReplicas(distcc)
	if err != nil {
		return state, errors.Wrap(err, "cannot determine desired replicas for distcc")
	}
	// Update the number of replicas, in case it doesn't match the desired
	if needScale(distcc, deploy, desiredReplicas) {
		new := newDeployment(distcc, &desiredReplicas)
		_, err = c.kubeclientset.AppsV1beta2().Deployments(distcc.Namespace).Update(new)

		// If an error occurs during Update, we'll requeue the item so we can
		// attempt processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		if err != nil {
			return state, k8ccerr.TransientError(err)
		}
		state.StatefulScaled = true
	}

	return state, nil
}

func (c *controller) desiredReplicas(distcc *k8ccv1alpha1.Distcc) (int32, error) {
	// Get all the matching DistccClaims and give them replicas if they are not expired
	now := time.Now()
	maxExpiration := now.Add(distcc.Spec.LeaseDuration.Duration)
	selector, err := metav1.LabelSelectorAsSelector(distcc.Spec.Selector)
	if err != nil {
		return 0, err
	}

	claims, err := c.claimsLister.DistccClaims(distcc.Namespace).List(selector)
	if err != nil {
		return 0, k8ccerr.TransientError(err)
	}
	var validClaims int32
	for _, claim := range claims {
		expiration := claim.Status.ExpirationTime
		if expiration != nil && expiration.Time.Before(maxExpiration) {
			validClaims++
		}
	}
	idealReplicas := validClaims * distcc.Spec.UserReplicas

	// Comply with min and max replicas
	if idealReplicas >= distcc.Spec.MaxReplicas {
		return distcc.Spec.MaxReplicas, nil
	}
	minReplicas := distcc.Spec.MinReplicas
	if minReplicas != nil && idealReplicas < *minReplicas {
		return *minReplicas, nil
	}
	return idealReplicas, nil
}

func (c *controller) syncService(distcc *k8ccv1alpha1.Distcc) (bool, error) {
	serviceName := distcc.Spec.ServiceName
	if serviceName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		return false, errors.New("spec.ServiceName must be specified")
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
		return updated, k8ccerr.TransientError(err)
	}

	// If the Service is not controlled by this Distcc resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(service, distcc) {
		msg := fmt.Sprintf(MessageResourceExists, service.Name)
		return updated, kit.EventfulError(errors.New(msg), kit.Event{
			Type:    corev1.EventTypeWarning,
			Reason:  ErrResourceExists,
			Message: msg,
		})
	}

	return updated, nil
}

func (c *controller) updateDistccStatus(distcc *k8ccv1alpha1.Distcc, updated distccUpdateState) error {
	// If the state hasn't changed, then don't update the distcc object
	if !updated.Any() {
		return nil
	}

	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	distccCopy := distcc.DeepCopy()
	now := conv.ToKubeTime(time.Now())
	distccCopy.Status.LastUpdateTime = now
	if updated.StatefulScaled {
		distccCopy.Status.LastScaleTime = now
	}
	_, err := c.k8ccclientset.K8ccV1alpha1().Distccs(distcc.Namespace).Update(distccCopy)
	return err
}

// newDeployment creates a new Deployment for a Distcc resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Deployment resource that 'owns' it. The number of replicas is optional.
func newDeployment(distcc *k8ccv1alpha1.Distcc, replicas *int32) *appsv1beta2.Deployment {
	return &appsv1beta2.Deployment{
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
		Spec: appsv1beta2.DeploymentSpec{
			Replicas: replicas,
			Selector: distcc.Spec.Selector,
			Template: distcc.Spec.Template,
		},
	}
}

func newService(distcc *k8ccv1alpha1.Distcc) *corev1.Service {
	var selector map[string]string
	if distcc.Spec.Selector != nil {
		selector = distcc.Spec.Selector.MatchLabels
	}
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
			Selector:  selector,
			ClusterIP: "None",
		},
	}
}

func needScale(distcc *k8ccv1alpha1.Distcc, deploy *appsv1beta2.Deployment, desiredReplicas int32) bool {
	if deploy.Spec.Replicas == nil {
		// The replicas are not set
		return true
	}
	currentReplicas := *deploy.Spec.Replicas

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
