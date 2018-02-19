package service

import (
	"context"
	"time"

	"github.com/go-kit/kit/endpoint"

	"github.com/mbrt/k8cc/pkg/data"
)

// Endpoints collects all the api endpoints in a single struct
type Endpoints struct {
	PutLeaseDistccEndpoint endpoint.Endpoint
	PutLeaseClientEndpoint endpoint.Endpoint
}

// MakeEndpoints creates the api endpoints, wiring in the given service
func MakeEndpoints(s Service) Endpoints {
	return Endpoints{
		PutLeaseDistccEndpoint: MakePutLeaseDistccEndpoint(s),
		PutLeaseClientEndpoint: MakePutLeaseClientEndpoint(s),
	}
}

// MakePutLeaseDistccEndpoint creates an endpoint for the LeaseDistcc service
func MakePutLeaseDistccEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(putLeaseDistccRequest)
		lu, err := s.LeaseDistcc(ctx, data.User(req.User), data.Tag{Namespace: req.Namespace, Name: req.Tag})
		roundTimestamp(&lu.Expiration) // prevent the ugly Json timestamp with nanoseconds
		return putLeaseDistccResponse{lu, err}, nil
	}
}

// MakePutLeaseClientEndpoint creates an endpoint for the LeaseDistccClient service
func MakePutLeaseClientEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(putLeaseClientRequest)
		lu, err := s.LeaseClient(ctx, data.User(req.User), data.Tag{Namespace: req.Namespace, Name: req.Tag})
		roundTimestamp(&lu.Expiration) // prevent the ugly Json timestamp with nanoseconds
		return putLeaseClientResponse{lu, err}, nil
	}
}

type putLeaseDistccRequest struct {
	User      string
	Namespace string
	Tag       string
}

type putLeaseDistccResponse struct {
	Lease Lease `json:"lease,omitempty"`
	Err   error `json:"error,omitempty"`
}

func (r putLeaseDistccResponse) error() error { return r.Err }

type putLeaseClientRequest struct {
	User      string
	Namespace string
	Tag       string
}

type putLeaseClientResponse struct {
	Lease Lease `json:"lease,omitempty"`
	Err   error `json:"error,omitempty"`
}

func (r putLeaseClientResponse) error() error { return r.Err }

func roundTimestamp(t *time.Time) {
	*t = t.Round(time.Second)
}
