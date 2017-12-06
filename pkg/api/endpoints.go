package api

import (
	"context"

	"github.com/go-kit/kit/endpoint"

	"github.com/mbrt/k8cc/pkg/data"
)

// Endpoints collects all the api endpoints in a single struct
type Endpoints struct {
	PutLeaseUserEndpoint endpoint.Endpoint
}

// MakeEndpoints creates the api endpoints, wiring in the given service
func MakeEndpoints(s Service) Endpoints {
	return Endpoints{
		PutLeaseUserEndpoint: MakePutLeaseUserEndpoint(s),
	}
}

// MakePutLeaseUserEndpoint creates an endpoint for the GetHosts service
func MakePutLeaseUserEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(putLeaseUserRequest)
		t, err := s.LeaseUser(ctx, data.User(req.User), data.Tag(req.Tag))
		return putLeaseUserResponse{t, err}, nil
	}
}

type putLeaseUserRequest struct {
	User string
	Tag  string
}

type putLeaseUserResponse struct {
	Lease Lease `json:"lease,omitempty"`
	Err   error `json:"error,omitempty"`
}

func (r putLeaseUserResponse) error() error { return r.Err }
