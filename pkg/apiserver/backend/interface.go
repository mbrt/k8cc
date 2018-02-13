package backend

import (
	"context"

	"github.com/mbrt/k8cc/pkg/data"
)

// Host is a hostname that can be used to connect to the requested service
type Host string

// Backend takes action in the cluster to satisfy resource requests.
//
// It is used to separate the api server frontend from the actual logic.
type Backend interface {
	LeaseDistcc(ctx context.Context, u data.User, t data.Tag) (Host, error)
}
