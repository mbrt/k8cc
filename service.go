package k8cc

import (
	"context"
	"net"
)

type Service interface {
	GetHosts(ctx context.Context, tag string) ([]net.IP, error)
}

func NewService(d Deployer) Service {
	return service{d}
}

type service struct {
	dep Deployer
}

func (s service) GetHosts(ctx context.Context, tag string) ([]net.IP, error) {
	return s.dep.Deployments(ctx, tag)
}
