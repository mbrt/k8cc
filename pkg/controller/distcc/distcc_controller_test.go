package distcc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
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
	obj, err := c.k8ccclientset.K8ccV1alpha1().Distccs(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	return c.controller.Sync(obj)
}

func newController(objects ...runtime.Object) *controllerTest {
	kubeclientset := fakekubeclient.NewSimpleClientset()
	k8ccclientset := fakeclient.NewSimpleClientset(objects...)
	distccInformerFactory := informers.NewSharedInformerFactory(k8ccclientset, 0)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeclientset, 0)
	deployInformer := kubeInformerFactory.Apps().V1().Deployments()
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	claimInformer := distccInformerFactory.K8cc().V1alpha1().DistccClaims()
	distccInformer := distccInformerFactory.K8cc().V1alpha1().Distccs()
	// Make test deterministic, avoiding time.Now()
	now, _ := time.Parse("2006/01/02 15:04", "2018/03/08 17:00")

	ctrl := &controllerTest{
		&controller{
			kubeclientset: kubeclientset,
			k8ccclientset: k8ccclientset,
			deployLister:  deployInformer.Lister(),
			serviceLister: serviceInformer.Lister(),
			distccsLister: distccInformer.Lister(),
			claimsLister:  claimInformer.Lister(),
		},
		distccInformerFactory,
		now,
	}
	ctrl.controller.now = func() time.Time { return ctrl.Now() }
	return ctrl
}

func TestDeployRefCheck(t *testing.T) {
	distcc := &k8ccv1alpha1.Distcc{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev",
			Namespace: "default",
			Labels:    map[string]string{"track": "unstable"},
		},
	}
	ctrl := newController(distcc)

	// Start the controller
	stopCh := make(chan struct{})
	ctrl.Start(stopCh)
	defer close(stopCh)

	// Sync a distcc without a deployment name fails
	sync, err := ctrl.Sync("default", "dev")
	assert.False(t, sync)
	assert.NotNil(t, err)
	event, ok := kit.HasEvent(err)
	assert.True(t, ok)
	assert.Equal(t, event.Reason, ErrDeployNotSpecified)
}

func TestCreateObjects(t *testing.T) {
	deployLabels := map[string]string{"track": "unstable"}
	distcc := &k8ccv1alpha1.Distcc{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev",
			Namespace: "default",
			Labels:    deployLabels,
		},
		Spec: k8ccv1alpha1.DistccSpec{
			DeploymentName: "dev-deploy",
			ServiceName:    "dev-svc",
			Selector: &metav1.LabelSelector{
				MatchLabels: deployLabels,
			},
			MaxReplicas:   8,
			UserReplicas:  3,
			LeaseDuration: metav1.Duration{Duration: 10 * time.Minute},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: deployLabels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Name: "distcc-container"},
					},
				},
			},
		},
	}
	ctrl := newController(distcc)

	// Start the controller
	stopCh := make(chan struct{})
	ctrl.Start(stopCh)
	defer close(stopCh)

	// Sync a distcc will create the deployment and the service
	sync, err := ctrl.Sync("default", "dev")
	assert.True(t, sync)
	assert.Nil(t, err)

	// Check the deployment
	deploy, err := ctrl.kubeclientset.AppsV1().Deployments("default").Get("dev-deploy", metav1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, "dev-deploy", deploy.Name)
	assert.True(t, metav1.IsControlledBy(deploy, distcc))
	assert.Equal(t, deployLabels, deploy.Labels)
	assert.Equal(t, int32(0), *deploy.Spec.Replicas)
	assert.Equal(t, distcc.Spec.Template, deploy.Spec.Template)

	// Check the service
	service, err := ctrl.kubeclientset.CoreV1().Services("default").Get("dev-svc", metav1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, "dev-svc", service.Name)
	assert.True(t, metav1.IsControlledBy(service, distcc))
	assert.Equal(t, deployLabels, service.Labels)
}
