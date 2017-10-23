package k8cc

import (
	"context"
	"net"

	"github.com/pkg/errors"
	//	appsv1beta1 "k8s.io/api/apps/v1beta1"
	//	apiv1 "k8s.io/api/core/v1"
	//	kerrors "k8s.io/apimachinery/pkg/api/errors"
	//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Deployer interface {
	Deployments(ctx context.Context, tag string) ([]net.IP, error)
}

func NewKubeDeployer() (Deployer, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, errors.Wrap(err, "error in cluster config")
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create client")
	}
	return kubeDeployer{clientset}, nil
}

type kubeDeployer struct {
	clientset *kubernetes.Clientset
}

func (d kubeDeployer) Deployments(ctx context.Context, tag string) ([]net.IP, error) {
	panic("not implemented")
}
