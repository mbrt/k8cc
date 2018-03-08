package distccclaim

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclient "k8s.io/client-go/kubernetes/fake"

	k8ccv1alpha1 "github.com/mbrt/k8cc/pkg/apis/k8cc.io/v1alpha1"
	fakeclient "github.com/mbrt/k8cc/pkg/client/clientset/versioned/fake"
	informers "github.com/mbrt/k8cc/pkg/client/informers/externalversions"
	"github.com/mbrt/k8cc/pkg/controller/kit"
)

type controllerTest struct {
	*controller
	informerFactory informers.SharedInformerFactory
	now             time.Time
}

func (c *controllerTest) Start(stopCh <-chan struct{}) {
	c.informerFactory.Start(stopCh)
	c.informerFactory.WaitForCacheSync(stopCh)
}

func (c *controllerTest) Now() time.Time {
	return c.now
}

// Sync calls the controller Sync with passing the most up to date version of the resource
func (c *controllerTest) Sync(namespace, name string) (bool, error) {
	obj, err := c.k8ccclientset.K8ccV1alpha1().DistccClaims(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	return c.controller.Sync(obj)
}

func newController(objects ...runtime.Object) *controllerTest {
	kubeclientset := fakekubeclient.NewSimpleClientset()
	k8ccclientset := fakeclient.NewSimpleClientset(objects...)
	distccInformerFactory := informers.NewSharedInformerFactory(k8ccclientset, 0)
	claimInformer := distccInformerFactory.K8cc().V1alpha1().DistccClaims()
	distccInformer := distccInformerFactory.K8cc().V1alpha1().Distccs()
	// Make test deterministic, avoiding time.Now()
	now, _ := time.Parse("2006/01/02 15:04", "2018/03/08 17:00")

	ctrl := &controllerTest{
		&controller{
			kubeclientset: kubeclientset,
			k8ccclientset: k8ccclientset,
			distccsLister: distccInformer.Lister(),
			claimsLister:  claimInformer.Lister(),
		},
		distccInformerFactory,
		now,
	}
	ctrl.controller.now = func() time.Time { return ctrl.Now() }
	return ctrl
}

func TestDistccRefCheck(t *testing.T) {
	claim := &k8ccv1alpha1.DistccClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foobar",
			Namespace: "default",
			Labels:    map[string]string{"track": "stable"},
		},
		Spec: k8ccv1alpha1.DistccClaimSpec{
			DistccName: "",
		},
	}
	distcc := &k8ccv1alpha1.Distcc{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev",
			Namespace: "default",
			Labels:    map[string]string{"track": "unstable"},
		},
	}
	ctrl := newController(claim, distcc)

	// Start the controller
	stopCh := make(chan struct{})
	ctrl.Start(stopCh)
	defer close(stopCh)

	// Sync a claim without a distcc name fails
	sync, err := ctrl.Sync("default", "foobar")
	assert.False(t, sync)
	assert.NotNil(t, err)
	event, ok := kit.HasEvent(err)
	assert.True(t, ok)
	assert.Equal(t, event.Reason, ErrDistccNotFound)

	// Update the ref to use a "master" distcc (non-existent)
	claim.Spec.DistccName = "master"
	_, err = ctrl.k8ccclientset.K8ccV1alpha1().DistccClaims("default").Update(claim)
	assert.Nil(t, err)

	// Sync a claim with a wrong distcc ref fails
	sync, err = ctrl.Sync("default", "foobar")
	assert.False(t, sync)
	assert.NotNil(t, err)
	event, ok = kit.HasEvent(err)
	assert.True(t, ok)
	assert.Equal(t, event.Reason, ErrDistccNotFound)

	// Update the ref to use the existing "dev" distcc
	claim.Spec.DistccName = "dev"
	_, err = ctrl.k8ccclientset.K8ccV1alpha1().DistccClaims("default").Update(claim)
	assert.Nil(t, err)

	// Sync a claim with different labels fails
	sync, err = ctrl.Sync("default", "foobar")
	assert.False(t, sync)
	assert.NotNil(t, err)
	event, ok = kit.HasEvent(err)
	assert.True(t, ok)
	assert.Equal(t, event.Reason, ErrDistccNotFound)
}

func TestDistccSetExpiration(t *testing.T) {
	labels := map[string]string{
		"track": "stable",
	}
	claim := &k8ccv1alpha1.DistccClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mike-master",
			Namespace: "default",
			Labels:    labels,
		},
		Spec: k8ccv1alpha1.DistccClaimSpec{
			DistccName: "master",
		},
	}
	distcc := &k8ccv1alpha1.Distcc{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "master",
			Namespace: "default",
			Labels:    labels,
		},
		Spec: k8ccv1alpha1.DistccSpec{
			LeaseDuration: metav1.Duration{Duration: 10 * time.Minute},
		},
	}
	ctrl := newController(claim, distcc)

	// Start the controller
	stopCh := make(chan struct{})
	ctrl.Start(stopCh)
	defer close(stopCh)

	// The distcc resource is present in the lister
	d, err := ctrl.distccsLister.Distccs("default").Get("master")
	assert.NotNil(t, d)
	assert.Nil(t, err)

	// The Sync should succeed now
	sync, err := ctrl.Sync("default", "mike-master")
	assert.Nil(t, err)
	assert.True(t, sync)

	// The result is that the expiration has been set
	claim, err = ctrl.k8ccclientset.K8ccV1alpha1().DistccClaims("default").Get("mike-master", metav1.GetOptions{})
	expectedExp := ctrl.now.Add(10 * time.Minute).Truncate(time.Second)
	assert.Nil(t, err)
	assert.Equal(t, expectedExp, claim.Status.ExpirationTime.Time)
}

func TestDistccDeleteWhenExpired(t *testing.T) {
	labels := map[string]string{
		"track": "stable",
	}
	claim := &k8ccv1alpha1.DistccClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mike-master",
			Namespace: "default",
			Labels:    labels,
		},
		Spec: k8ccv1alpha1.DistccClaimSpec{
			DistccName: "master",
		},
	}
	distcc := &k8ccv1alpha1.Distcc{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "master",
			Namespace: "default",
			Labels:    labels,
		},
		Spec: k8ccv1alpha1.DistccSpec{
			LeaseDuration: metav1.Duration{Duration: 10 * time.Minute},
		},
	}
	ctrl := newController(claim, distcc)

	// Start the controller
	stopCh := make(chan struct{})
	ctrl.Start(stopCh)
	defer close(stopCh)

	// Sync to set the expiration
	sync, err := ctrl.Sync("default", "mike-master")
	assert.Nil(t, err)
	assert.True(t, sync)

	// Check that the claim is still there
	_, err = ctrl.k8ccclientset.K8ccV1alpha1().DistccClaims("default").Get("mike-master", metav1.GetOptions{})
	assert.Nil(t, err)

	// Some time passes, but not enough for the expiration
	ctrl.now = ctrl.now.Add(6 * time.Minute)
	assert.Equal(t, ctrl.now, ctrl.controller.now())

	sync, err = ctrl.Sync("default", "mike-master")
	assert.Nil(t, err)
	assert.False(t, sync)

	// Check that the claim is still there
	_, err = ctrl.k8ccclientset.K8ccV1alpha1().DistccClaims("default").Get("mike-master", metav1.GetOptions{})
	assert.Nil(t, err)

	// Then we have an expiration
	ctrl.now = ctrl.now.Add(5 * time.Minute)
	sync, err = ctrl.Sync("default", "mike-master")
	assert.Nil(t, err)
	assert.False(t, sync)

	// Check that the claim is gone
	_, err = ctrl.k8ccclientset.K8ccV1alpha1().DistccClaims("default").Get("mike-master", metav1.GetOptions{})
	assert.True(t, kubeerr.IsNotFound(err))
}
