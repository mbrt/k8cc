package state

import (
	"time"

	"github.com/mbrt/k8cc/pkg/data"
)

// TagsStater contains the leases for all tags
type TagsStater interface {
	// State returns the state for the given tag
	TagState(t data.Tag) TagStater
	// Tags returns all the known tags
	Tags() []data.Tag
}

// TagStater holds the state of build hosts and leases to users
type TagStater interface {
	// SetLease sets a user's lease time to a list of hosts.
	// It implicitly removes an eventual pre-existing lease for that user.
	SetLease(u data.User, expire time.Time, hs []data.HostID)
	// RemoveLease removes a user lease if present
	RemoveLease(u data.User)
	// HostsUsage returns the number of users assigned to each build host, ordered by build host id
	HostsUsage(now time.Time) []int
}
