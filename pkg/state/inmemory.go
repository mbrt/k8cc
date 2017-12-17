package state

import (
	"sync"
	"time"

	"github.com/mbrt/k8cc/pkg/data"
)

// NewInMemoryState returns a TagStater backed by in-memomry storage
func NewInMemoryState() TagsStater {
	return &inMemoryState{
		map[data.Tag]*inMemoryTagState{},
		sync.Mutex{},
	}
}

type inMemoryState struct {
	tags map[data.Tag]*inMemoryTagState
	mut  sync.Mutex
}

func (s *inMemoryState) TagState(t data.Tag) TagStater {
	s.mut.Lock()
	defer s.mut.Unlock()

	if ts, ok := s.tags[t]; ok {
		return ts
	}
	ts := &inMemoryTagState{
		map[data.User]userLeaseInfo{},
		sync.Mutex{},
	}
	s.tags[t] = ts
	return ts
}

func (s *inMemoryState) Tags() []data.Tag {
	s.mut.Lock()
	defer s.mut.Unlock()

	res := []data.Tag{}
	for t := range s.tags {
		res = append(res, t)
	}
	return res
}

type inMemoryTagState struct {
	leases map[data.User]userLeaseInfo
	mut    sync.Mutex
}

type userLeaseInfo struct {
	expiration time.Time
	hosts      []data.HostID
}

func (tl *inMemoryTagState) SetLease(u data.User, expire time.Time, hs []data.HostID) {
	tl.mut.Lock()
	defer tl.mut.Unlock()
	tl.leases[u] = userLeaseInfo{expire, hs}
}

func (tl *inMemoryTagState) RemoveLease(u data.User) {
	tl.mut.Lock()
	defer tl.mut.Unlock()
	delete(tl.leases, u)
}

func (tl *inMemoryTagState) HostsUsage(now time.Time) []int {
	tl.mut.Lock()
	defer tl.mut.Unlock()

	result := []int{}
	for user, lease := range tl.leases {
		// remove expired users
		if now.After(lease.expiration) {
			delete(tl.leases, user)
		} else {
			// update the hosts used by this user
			for _, h := range lease.hosts {
				hostIdx := int(h)
				if len(result) <= hostIdx {
					// resize the result to fit the current host id
					result = append(result, make([]int, hostIdx-len(result)+1)...)
				}
				result[h]++
			}
		}
	}
	return result
}

func (tl *inMemoryTagState) Leases(now time.Time) []data.Lease {
	tl.mut.Lock()
	defer tl.mut.Unlock()

	result := []data.Lease{}
	for user, lease := range tl.leases {
		// remove expired users
		if now.After(lease.expiration) {
			delete(tl.leases, user)
		} else {
			lease := data.Lease{
				User:       user,
				Expiration: lease.expiration,
				Hosts:      lease.hosts,
			}
			result = append(result, lease)
		}
	}
	return result
}
