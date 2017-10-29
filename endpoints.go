package k8cc

import (
	"context"

	"github.com/go-kit/kit/endpoint"
)

// Endpoints collects all the api endpoints in a single struct
type Endpoints struct {
	GetHostsEndpoint endpoint.Endpoint
}

// MakeEndpoints creates the api endpoints, wiring in the given service
func MakeEndpoints(s Service) Endpoints {
	return Endpoints{
		GetHostsEndpoint: MakeGetHostsEndpoint(s),
	}
}

// MakeGetHostsEndpoint creates an endpoint for the GetHosts service
func MakeGetHostsEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(getHostsRequest)
		h, err := s.Hosts(ctx, req.Tag)
		return getHostsResponse{h, err}, nil
	}
}

type getHostsRequest struct {
	Tag string
}

type getHostsResponse struct {
	Hosts []Host `json:"hosts,omitempty"`
	Err   error  `json:"error,omitempty"`
}
