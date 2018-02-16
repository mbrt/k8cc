package backend

import (
	"context"
	"time"

	"github.com/mbrt/k8cc/pkg/data"
)

// Lease contains info about a lease for a specific user and tag
type Lease struct {
	// Expiration is the time at which the lease will end
	Expiration time.Time
	// Hosts is the list of hosts that can be used to access the resoure
	// being leased
	Hosts []string
}

// Backend takes action in the cluster to satisfy resource requests.
//
// It is used to separate the api server frontend from the actual logic.
type Backend interface {
	LeaseDistcc(ctx context.Context, u data.User, t data.Tag) (Lease, error)
}
