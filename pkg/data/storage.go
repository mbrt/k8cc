package data

import (
	"sync"
	"time"
)

// Storage stores user leases
type Storage interface {
	// GetLease returns a user's lease time for a specific tag, if present, nil otherwise
	GetLease(tag, user string) *time.Time
	// SetLease sets a user's lease time for a specific tag
	SetLease(tag, user string, expire time.Time, hosts []BuildHostID)
	// RemoveLease removes a user lease if present
	RemoveLease(tag, user string)
	// NumActiveUsers returns the number of active users for a certain tag
	NumActiveUsers(tag string, now time.Time) int
	// Usage returns the number of users assigned to each build host, ordered by build host id
	Usage(tag string, now time.Time) []int
}

// NewInMemoryStorage returns a lease storage that keeps the information in memory
func NewInMemoryStorage() Storage {
	return &inMemoryStorage{
		map[string]tagUsersLease{},
		sync.Mutex{},
	}
}

type inMemoryStorage struct {
	tags map[string]tagUsersLease
	mut  sync.Mutex
}

func (s *inMemoryStorage) GetLease(tag, user string) *time.Time {
	s.mut.Lock()
	defer s.mut.Unlock()

	if tagLease, ok := s.tags[tag]; !ok {
		return tagLease.GetLease(user)
	}
	return nil
}

func (s *inMemoryStorage) SetLease(tag, user string, expire time.Time, hosts []BuildHostID) {
	s.mut.Lock()
	defer s.mut.Unlock()

	if _, ok := s.tags[tag]; !ok {
		s.tags[tag] = tagUsersLease{}
	}
	s.tags[tag][user] = userLeaseInfo{expire, hosts}
}

func (s *inMemoryStorage) RemoveLease(tag, user string) {
	s.mut.Lock()
	defer s.mut.Unlock()

	if tl, ok := s.tags[tag]; ok {
		delete(tl, user)
	}
}

func (s *inMemoryStorage) NumActiveUsers(tag string, now time.Time) int {
	s.mut.Lock()
	defer s.mut.Unlock()

	if tl, ok := s.tags[tag]; ok {
		return tl.NumActiveUsers(now)
	}
	return 0
}

func (s *inMemoryStorage) Usage(tag string, now time.Time) []int {
	s.mut.Lock()
	defer s.mut.Unlock()

	if tl, ok := s.tags[tag]; ok {
		return tl.Usage(now)
	}
	return nil
}

// tagUsersLease maps users to expiration times
type tagUsersLease map[string]userLeaseInfo

type userLeaseInfo struct {
	expiration time.Time
	hosts      []BuildHostID
}

func (tl tagUsersLease) GetLease(user string) *time.Time {
	if lease, ok := tl[user]; ok {
		return &lease.expiration
	}
	return nil
}

func (tl tagUsersLease) NumActiveUsers(now time.Time) int {
	for user, lease := range tl {
		if lease.expiration.Before(now) {
			delete(tl, user)
		}
	}
	return len(tl)
}

func (tl tagUsersLease) Usage(now time.Time) []int {
	result := []int{}
	for user, lease := range tl {
		// remove expired users
		if lease.expiration.Before(now) {
			delete(tl, user)
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
