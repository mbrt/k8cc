package service

import (
	"context"
	"time"

	"github.com/mbrt/k8cc/pkg/apiserver/backend"
	"github.com/mbrt/k8cc/pkg/data"
)

var (
	// ErrCanceled is used when the request cannot be satisfied on time
	ErrCanceled = backend.ErrCanceled
	// ErrNotFound is used when the given resource was not found
	ErrNotFound = backend.ErrNotFound
)

// Service is an interface that implements all the APIs.
type Service interface {
	LeaseDistcc(ctx context.Context, u data.User, t data.Tag) (Lease, error)
	DeleteDistcc(ctx context.Context, u data.User, t data.Tag) error
	LeaseClient(ctx context.Context, u data.User, t data.Tag) (Lease, error)
	DeleteClient(ctx context.Context, u data.User, t data.Tag) error
}

// NewService creates the API service
func NewService(b backend.Backend) Service {
	return service{b}
}

// Lease contains info about a lease for a specific user and tag
type Lease struct {
	Expiration time.Time `json:"expiration"`
	Endpoints  []string  `json:"endpoints,omitempty"`
	NodePort   int       `json:"nodePort,omitempty"`
	Replicas   int       `json:"replicas,omitempty"`
}

type service struct {
	backend backend.Backend
}

func (s service) LeaseDistcc(ctx context.Context, u data.User, t data.Tag) (Lease, error) {
	lease, err := s.backend.LeaseDistcc(ctx, u, t)
	if err != nil {
		return Lease{}, err
	}
	result := Lease{
		Expiration: lease.Expiration,
		Endpoints:  lease.Endpoints,
		Replicas:   lease.Replicas,
	}
	return result, err
}

func (s service) DeleteDistcc(ctx context.Context, u data.User, t data.Tag) error {
	return s.backend.DeleteDistcc(ctx, u, t)
}

func (s service) LeaseClient(ctx context.Context, u data.User, t data.Tag) (Lease, error) {
	lease, err := s.backend.LeaseClient(ctx, u, t)
	if err != nil {
		return Lease{}, err
	}
	result := Lease{
		Expiration: lease.Expiration,
		NodePort:   lease.NodePort,
	}
	return result, err
}

func (s service) DeleteClient(ctx context.Context, u data.User, t data.Tag) error {
	return s.backend.DeleteClient(ctx, u, t)
}
