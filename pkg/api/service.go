package api

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/mbrt/k8cc/pkg/controller"
	"github.com/mbrt/k8cc/pkg/kube"
)

var (
	// ErrCanceled is used when the request cannot be satisfied on time
	ErrCanceled = errors.New("timeout or canceled")
)

// Service is an interface that implements all the APIs.
type Service interface {
	LeaseUser(ctx context.Context, user, tag string) (Lease, error)
}

// NewService creates the API service
func NewService(d kube.Deployer, c controller.Controller) Service {
	return service{d, c}
}

// Lease contains info about a lease for a specific user and tag
type Lease struct {
	Expiration time.Time `json:"expiration"`
	Hosts      []string  `json:"hosts"`
}

type service struct {
	dep        kube.Deployer
	controller controller.Controller
}

func (s service) LeaseUser(ctx context.Context, user, tag string) (Lease, error) {
	lease, err := s.controller.TagController(tag).LeaseUser(ctx, user, time.Now())
	result := Lease{
		Expiration: lease.Expiration,
		Hosts:      lease.Hosts,
	}
	return result, err
}
