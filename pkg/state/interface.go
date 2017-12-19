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
	// Leases returns all the active leases
	Leases(now time.Time) []data.Lease
}

// Loader returns the full state of the leases
type Loader interface {
	// Load returns all the leases
	Load() ([]data.TagLeases, error)
}

// LoadFrom returns an in-memory TagStater, loaded with the given loader
func LoadFrom(stater TagsStater, loader Loader) error {
	var leases []data.TagLeases
	var err error
	if leases, err = loader.Load(); err != nil {
		return err
	}
	for _, tl := range leases {
		ts := stater.TagState(tl.Tag)
		for _, lease := range tl.Leases {
			ts.SetLease(lease.User, lease.Expiration, lease.Hosts)
		}
	}
	return nil
}
