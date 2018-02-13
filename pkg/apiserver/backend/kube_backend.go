package backend

import (
	"context"

	"github.com/mbrt/k8cc/pkg/controller"
	"github.com/mbrt/k8cc/pkg/data"
)

// NewKubeBackend creates a backend that applies the configuration to a Kubernetes cluster
func NewKubeBackend(sharedclient *controller.SharedClient) Backend {
	return &kubeBackend{}
}

type kubeBackend struct{}

func (b *kubeBackend) LeaseDistcc(ctx context.Context, u data.User, t data.Tag) (Host, error) {
	return "", nil
}
