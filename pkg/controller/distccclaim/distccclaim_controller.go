package distccclaim

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	k8ccv1alpha1 "github.com/mbrt/k8cc/pkg/apis/k8cc.io/v1alpha1"
	clientset "github.com/mbrt/k8cc/pkg/client/clientset/versioned"
	listers "github.com/mbrt/k8cc/pkg/client/listers/k8cc/v1alpha1"
	sharedctr "github.com/mbrt/k8cc/pkg/controller"
	"github.com/mbrt/k8cc/pkg/controller/kit"
	"github.com/mbrt/k8cc/pkg/conv"
	k8ccerr "github.com/mbrt/k8cc/pkg/errors"
	labels "k8s.io/apimachinery/pkg/labels"
)

const (
	// ErrDistccNotFound is used as part of the Event 'reason' when a DistccClaim fails
	// to sync due to a dangling reference to the Distcc object
	ErrDistccNotFound = "ErrDistccNotFound"
)

const controllerAgentName = "k8cc-distcc-claim-controller"

// controller controls DistccClaim objects
type controller struct {
	kubeclientset kubernetes.Interface
	k8ccclientset clientset.Interface
	distccsLister listers.DistccLister
	claimsLister  listers.DistccClaimLister
}

// NewController creates a controller for DistccClaims using the given shared client connection.
func NewController(sharedClient *sharedctr.SharedClient) sharedctr.Controller {
	claimInformer := sharedClient.DistccInformerFactory.K8cc().V1alpha1().DistccClaims()
	distccInformer := sharedClient.DistccInformerFactory.K8cc().V1alpha1().Distccs()

	handler := controller{
		kubeclientset: sharedClient.KubeClientset,
		k8ccclientset: sharedClient.K8ccClientset,
		distccsLister: distccInformer.Lister(),
		claimsLister:  claimInformer.Lister(),
	}

	return kit.NewController(
		&handler,
		sharedClient.KubeClientset,
		claimInformer.Informer(),
		[]cache.SharedInformer{
			distccInformer.Informer(),
		},
	)
}

func (c *controller) AgentName() string {
	return controllerAgentName
}

func (c *controller) Sync(object runtime.Object) (bool, error) {
	claim := object.(*k8ccv1alpha1.DistccClaim)

	if claim.Spec.DistccName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		msg := "spec.distccName name must be specified"
		return false, kit.EventfulError(errors.New(msg), kit.Event{
			Type:    corev1.EventTypeWarning,
			Reason:  ErrDistccNotFound,
			Message: msg,
		})
	}

	// Get the corresponding Distcc
	distcc, err := c.distccsLister.Distccs(claim.Namespace).Get(claim.Spec.DistccName)
	if err != nil {
		// If the resource doesn't exist, we need to wait
		msg := fmt.Sprintf("referenced Distcc %s not present yet", claim.Spec.DistccName)
		return false, kit.EventfulError(errors.New(msg), kit.Event{
			Type:    corev1.EventTypeWarning,
			Reason:  ErrDistccNotFound,
			Message: msg,
		})
	}

	// Check that the distcc labels match, otherwise it will be impossible for the distcc
	// controller to take this claim into account
	if !labels.Set(distcc.Labels).AsSelector().Matches(labels.Set(claim.Labels)) {
		msg := fmt.Sprintf("metadata.Labels cannot be matched by the referenced Distcc %s",
			claim.Spec.DistccName)
		return false, kit.EventfulError(errors.New(msg), kit.Event{
			Type:    corev1.EventTypeWarning,
			Reason:  ErrDistccNotFound,
			Message: msg,
		})
	}

	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	claim = claim.DeepCopy()

	// Check expiration time, and in case delete the resource
	changed := c.updateExpiration(claim, distcc)
	if c.isExpired(claim) {
		err = c.k8ccclientset.K8ccV1alpha1().DistccClaims(claim.Namespace).Delete(claim.Name, nil)
		return false, k8ccerr.TransientError(err)
	}

	if changed {
		glog.V(3).Infof("%s: updated expiration to %s", claim.Name, claim.Status.ExpirationTime)
		_, err = c.k8ccclientset.K8ccV1alpha1().DistccClaims(claim.Namespace).Update(claim)
		return true, k8ccerr.TransientError(err)
	}

	return false, nil
}

func (c *controller) OnControlledObjectUpdate(object interface{}) (interface{}, error) {
	// No custom discovery required
	return nil, nil
}

func (c *controller) CustomResourceKind() string {
	return "DistccClaim"
}

func (c *controller) CustomResourceInstance(namespace, name string) (runtime.Object, error) {
	return c.claimsLister.DistccClaims(namespace).Get(name)
}

func (c *controller) NeedPeriodicSync() bool {
	return true
}

func (c *controller) updateExpiration(claim *k8ccv1alpha1.DistccClaim, distcc *k8ccv1alpha1.Distcc) bool {
	now := time.Now()
	maxExpiration := now.Add(distcc.Spec.LeaseDuration.Duration)
	// If expiration is not set, set it automatically
	// If the expiration exceedes the maximum possible one (namely now + expiration time)
	// then reduce it to that value
	if claim.Status.ExpirationTime == nil || claim.Status.ExpirationTime.After(maxExpiration) {
		claim.Status.ExpirationTime = conv.ToKubeTime(maxExpiration)
		return true
	}
	return false
}

func (c *controller) isExpired(claim *k8ccv1alpha1.DistccClaim) bool {
	return time.Now().After(claim.Status.ExpirationTime.Time)
}
