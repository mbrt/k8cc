package service

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/mbrt/k8cc/pkg/apiserver/backend"
	"github.com/mbrt/k8cc/pkg/data"
)

var (
	// ErrCanceled is used when the request cannot be satisfied on time
	ErrCanceled = errors.New("timeout or canceled")
)

// Service is an interface that implements all the APIs.
type Service interface {
	LeaseUser(ctx context.Context, u data.User, t data.Tag) (Lease, error)
}

// NewService creates the API service
func NewService(b backend.Backend) Service {
	return service{b}
}

// Lease contains info about a lease for a specific user and tag
type Lease struct {
	Expiration time.Time `json:"expiration"`
	Hosts      []string  `json:"hosts"`
}

type service struct {
	backend backend.Backend
}

func (s service) LeaseUser(ctx context.Context, u data.User, t data.Tag) (Lease, error) {
	host, err := s.backend.LeaseDistcc(ctx, u, t)
	if err != nil {
		return Lease{}, err
	}
	result := Lease{
		Hosts: []string{string(host)},
	}
	return result, err
}
