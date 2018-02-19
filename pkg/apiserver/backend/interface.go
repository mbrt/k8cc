package backend

import (
	"context"
	"time"

	"github.com/mbrt/k8cc/pkg/data"
)

// DistccLease contains info about a lease for a specific user and tag.
type DistccLease struct {
	// Expiration is the time at which the lease will end.
	Expiration time.Time
	// Endpoints is the list of hosts that can be used to access the resoure
	// being leased.
	Endpoints []string
	// Replicas is the number of replicas sitting behind the endpoints.
	//
	// This might be useful to determine the number of replicas behind the
	// given endpoints.
	Replicas int
}

// ClientLease contains info about a lease for a specific user and tag.
type ClientLease struct {
	// Expiration is the time at which the lease will end.
	Expiration time.Time
	// NodePort is the port allocated for the service.
	//
	// The service will be accessible from any host with an external
	// interface running on the cluster.
	NodePort int
}

// Backend takes action in the cluster to satisfy resource requests.
//
// It is used to separate the api server frontend from the actual logic.
type Backend interface {
	LeaseDistcc(ctx context.Context, u data.User, t data.Tag) (DistccLease, error)
	LeaseClient(ctx context.Context, u data.User, t data.Tag) (ClientLease, error)
}
