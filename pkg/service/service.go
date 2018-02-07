package service

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/mbrt/k8cc/pkg/algo"
	"github.com/mbrt/k8cc/pkg/controller/distccold"
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
func NewService(c algo.Controller, k distccold.Operator) Service {
	return service{c, k}
}

// Lease contains info about a lease for a specific user and tag
type Lease struct {
	Expiration time.Time `json:"expiration"`
	Hosts      []string  `json:"hosts"`
}

type service struct {
	controller algo.Controller
	operator   distccold.Operator
}

func (s service) LeaseUser(ctx context.Context, u data.User, t data.Tag) (Lease, error) {
	lease, err := s.controller.TagController(t).LeaseUser(ctx, u, time.Now())
	if err != nil {
		return Lease{}, err
	}
	err = s.operator.NotifyUpdated(t)
	result := Lease{
		Expiration: lease.Expiration,
		Hosts:      lease.Hosts,
	}
	return result, err
}
