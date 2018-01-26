package controller

import (
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	clientset "github.com/mbrt/k8cc/pkg/client/clientset/versioned"
	informers "github.com/mbrt/k8cc/pkg/client/informers/externalversions"
)

// SharedClient provides a shared connection for all operators
type SharedClient struct {
	KubeClientset         kubernetes.Interface
	K8ccClientset         clientset.Interface
	KubeInformerFactory   kubeinformers.SharedInformerFactory
	DistccInformerFactory informers.SharedInformerFactory
}

// NewSharedClient creates a new connection to the kubernetes master.
//
// If kubecfg and masterURL are empty, defaults to in-cluster configuration. This should be
// shared with as many controllers as possible.
func NewSharedClient(masterURL, kubecfg string) (*SharedClient, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubecfg)
	if err != nil {
		return nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	distccInformerFactory := informers.NewSharedInformerFactory(client, time.Second*30)
	return &SharedClient{
		KubeClientset:         kubeClient,
		K8ccClientset:         client,
		KubeInformerFactory:   kubeInformerFactory,
		DistccInformerFactory: distccInformerFactory,
	}, nil
}

// Run starts the client connection, and sync the caches.
func (c *SharedClient) Run(stopCh <-chan struct{}) error {
	go c.KubeInformerFactory.Start(stopCh)
	go c.DistccInformerFactory.Start(stopCh)

	<-stopCh
	return nil
}
