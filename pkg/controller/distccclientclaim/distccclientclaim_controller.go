package distccclient

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1beta2"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	k8ccv1alpha1 "github.com/mbrt/k8cc/pkg/apis/k8cc.io/v1alpha1"
	clientset "github.com/mbrt/k8cc/pkg/client/clientset/versioned"
	listers "github.com/mbrt/k8cc/pkg/client/listers/k8cc/v1alpha1"
	sharedctr "github.com/mbrt/k8cc/pkg/controller"
	"github.com/mbrt/k8cc/pkg/controller/kit"
	"github.com/mbrt/k8cc/pkg/conv"
	k8ccerr "github.com/mbrt/k8cc/pkg/errors"
)

const controllerAgentName = "k8cc-distcc-client-controller"

const (
	// ErrResourceExists is used as part of the Event 'reason' when a Distcc fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"
	// ErrAmbiguousLabels is used as part of the Event 'reason' when a Distcc fails
	// to sync due to multiple deployment satisfying the label selector.
	ErrAmbiguousLabels = "ErrAmbiguousMatch"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by DistccClientClaim"

	// UserLabel is the label used to identify a resource given to a particular user
	UserLabel = "k8cc.io/username"

	// SSHPort is the port used by the ssh daemon
	SSHPort = 22
)

// NewController creates a new Controller
func NewController(sharedClient *sharedctr.SharedClient) sharedctr.Controller {
	deployInformer := sharedClient.KubeInformerFactory.Apps().V1beta2().Deployments()
	serviceInformer := sharedClient.KubeInformerFactory.Core().V1().Services()
	clientInformer := sharedClient.DistccInformerFactory.K8cc().V1alpha1().DistccClients()
	claimInformer := sharedClient.DistccInformerFactory.K8cc().V1alpha1().DistccClientClaims()

	handler := controller{
		kubeclientset:       sharedClient.KubeClientset,
		k8ccclientset:       sharedClient.K8ccClientset,
		deploymentsLister:   deployInformer.Lister(),
		servicesLister:      serviceInformer.Lister(),
		distccclientsLister: clientInformer.Lister(),
		distccclaimsLister:  claimInformer.Lister(),
	}

	return kit.NewController(
		&handler,
		sharedClient.KubeClientset,
		claimInformer.Informer(),
		[]cache.SharedInformer{
			deployInformer.Informer(),
			serviceInformer.Informer(),
			clientInformer.Informer(),
		},
	)
}

type controller struct {
	kubeclientset       kubernetes.Interface
	k8ccclientset       clientset.Interface
	deploymentsLister   appslisters.DeploymentLister
	servicesLister      corelisters.ServiceLister
	distccclientsLister listers.DistccClientLister
	distccclaimsLister  listers.DistccClientClaimLister
}

func (c *controller) AgentName() string {
	return controllerAgentName
}

func (c *controller) Sync(object runtime.Object) (bool, error) {
	claim := object.(*k8ccv1alpha1.DistccClientClaim)
	namespace := claim.Namespace
	name := claim.Name

	// Get the corresponding DistccClient resource
	dclient, err := c.distccclientsLister.DistccClients(namespace).Get(claim.Spec.DistccClientName)
	if err != nil {
		if kubeerr.IsNotFound(err) {
			// The referring DistccClient is invalid, stop processing this element
			return false, errors.Errorf("distccclient '%s' is not present", claim.Spec.DistccClientName)
		}
		return false, k8ccerr.TransientError(err)
	}

	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	claim = claim.DeepCopy()

	// If the expiration time is not set, set it automatically
	changed := componentsChange{}
	if changed.Expiration = c.updateExpiration(claim, dclient); changed.Expiration {
		glog.V(3).Infof("updated expiration to %s", claim.Status.ExpirationTime)
	}

	// Check expiration time, and in case delete the resource
	if c.isExpired(claim) {
		err = c.k8ccclientset.K8ccV1alpha1().DistccClientClaims(namespace).Delete(name, nil)
		return false, k8ccerr.TransientError(err)
	}

	// Check the related Deployment
	changed.Deploy, err = c.syncDeploy(claim, dclient)
	if err != nil {
		return changed.Any(), err
	}

	// Check the related Service
	changed.Service, err = c.syncService(claim, dclient)
	if err != nil {
		return changed.Any(), err
	}

	if changed.Any() {
		_, err = c.k8ccclientset.K8ccV1alpha1().DistccClientClaims(namespace).Update(claim)
		return true, k8ccerr.TransientError(err)
	}

	return false, nil
}

func (c *controller) CustomResourceKind() string {
	return "DistccClientClaim"
}

func (c *controller) CustomResourceInstance(namespace, name string) (runtime.Object, error) {
	return c.distccclaimsLister.DistccClientClaims(namespace).Get(name)
}

func (c *controller) NeedPeriodicSync() bool {
	return true
}

func (c *controller) OnControlledObjectUpdate(object interface{}) (interface{}, error) {
	// No custom discovery required
	return nil, nil
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
			return false, kit.EventfulError(errors.New(msg), kit.Event{
				Type:    corev1.EventTypeWarning,
				Reason:  ErrAmbiguousLabels,
				Message: msg,
			})
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
		return false, kit.EventfulError(errors.New(msg), kit.Event{
			Type:    corev1.EventTypeWarning,
			Reason:  ErrResourceExists,
			Message: msg,
		})
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
			return false, kit.EventfulError(errors.New(msg), kit.Event{
				Type:    corev1.EventTypeWarning,
				Reason:  ErrAmbiguousLabels,
				Message: msg,
			})
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
		return false, kit.EventfulError(errors.New(msg), kit.Event{
			Type:    corev1.EventTypeWarning,
			Reason:  ErrResourceExists,
			Message: msg,
		})
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

func (c *controller) isExpired(claim *k8ccv1alpha1.DistccClientClaim) bool {
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
