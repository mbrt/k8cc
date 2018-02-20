package service

import (
	"context"
	"time"

	"github.com/go-kit/kit/endpoint"

	"github.com/mbrt/k8cc/pkg/data"
)

// Endpoints collects all the api endpoints in a single struct
type Endpoints struct {
	PutLeaseDistccEndpoint    endpoint.Endpoint
	DeleteLeaseDistccEndpoint endpoint.Endpoint
	PutLeaseClientEndpoint    endpoint.Endpoint
}

// MakeEndpoints creates the api endpoints, wiring in the given service
func MakeEndpoints(s Service) Endpoints {
	return Endpoints{
		PutLeaseDistccEndpoint:    MakePutLeaseDistccEndpoint(s),
		DeleteLeaseDistccEndpoint: MakeDeleteLeaseDistccEndpoint(s),
		PutLeaseClientEndpoint:    MakePutLeaseClientEndpoint(s),
	}
}

// MakePutLeaseDistccEndpoint creates an endpoint for the LeaseDistcc service
func MakePutLeaseDistccEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(leaseRequest)
		lu, err := s.LeaseDistcc(ctx, data.User(req.User), data.Tag{Namespace: req.Namespace, Name: req.Tag})
		return newLeaseResponse(lu, err), nil
	}
}

// MakeDeleteLeaseDistccEndpoint creates an endpoint for the DeleteDistcc service
func MakeDeleteLeaseDistccEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(leaseRequest)
		err = s.DeleteDistcc(ctx, data.User(req.User), data.Tag{Namespace: req.Namespace, Name: req.Tag})
		return err, nil
	}
}

// MakePutLeaseClientEndpoint creates an endpoint for the LeaseDistccClient service
func MakePutLeaseClientEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(leaseRequest)
		lu, err := s.LeaseClient(ctx, data.User(req.User), data.Tag{Namespace: req.Namespace, Name: req.Tag})
		return newLeaseResponse(lu, err), nil
	}
}

type leaseRequest struct {
	User      string
	Namespace string
	Tag       string
}

type leaseResponse struct {
	Lease Lease `json:"lease,omitempty"`
	Err   error `json:"error,omitempty"`
}

func (r leaseResponse) error() error { return r.Err }

func newLeaseResponse(lease Lease, err error) leaseResponse {
	res := leaseResponse{
		Lease: lease,
		Err:   err,
	}
	// prevent the ugly Json timestamp with nanoseconds
	res.Lease.Expiration = roundTimestamp(lease.Expiration)
	return res
}

func roundTimestamp(t time.Time) time.Time {
	return t.Round(time.Second)
}
