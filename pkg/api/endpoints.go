package api

import (
	"context"

	"github.com/go-kit/kit/endpoint"
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
		t, err := s.LeaseUser(ctx, req.User, req.Tag)
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
