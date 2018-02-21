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
	DeleteLeaseClientEndpoint endpoint.Endpoint
}

// MakeEndpoints creates the api endpoints, wiring in the given service
func MakeEndpoints(s Service) Endpoints {
	return Endpoints{
		PutLeaseDistccEndpoint:    MakePutLeaseDistccEndpoint(s),
		DeleteLeaseDistccEndpoint: MakeDeleteLeaseDistccEndpoint(s),
		PutLeaseClientEndpoint:    MakePutLeaseClientEndpoint(s),
		DeleteLeaseClientEndpoint: MakeDeleteLeaseClientEndpoint(s),
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
		return newDeleteLeaseResponse(err), nil
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

// MakeDeleteLeaseClientEndpoint creates an endpoint for the DeleteClient service
func MakeDeleteLeaseClientEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(leaseRequest)
		err = s.DeleteClient(ctx, data.User(req.User), data.Tag{Namespace: req.Namespace, Name: req.Tag})
		return newDeleteLeaseResponse(err), nil
	}
}

// We have two options to return errors from the business logic.
//
// We could return the error via the endpoint itself. That makes certain things
// a little bit easier, like providing non-200 HTTP responses to the client. But
// Go kit assumes that endpoint errors are (or may be treated as)
// transport-domain errors. For example, an endpoint error will count against a
// circuit breaker error count.
//
// Therefore, it's often better to return service (business logic) errors in the
// response object. This means we have to do a bit more work in the HTTP
// response encoder to detect e.g. a not-found error and provide a proper HTTP
// status code. That work is done with the errorer interface, in transport.go.
// Response types that may contain business-logic errors implement that
// interface.

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

type deleteLeaseResponse struct {
	Err error `json:"error,omitempty"`
}

func (r deleteLeaseResponse) error() error { return r.Err }

func newDeleteLeaseResponse(err error) deleteLeaseResponse {
	return deleteLeaseResponse{
		Err: err,
	}
}

func roundTimestamp(t time.Time) time.Time {
	return t.Round(time.Second)
}
