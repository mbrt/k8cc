package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mbrt/k8cc/pkg/data"
)

func TestStorageSingleUser(t *testing.T) {
	state := NewInMemoryState()
	mstate := state.TagState(data.Tag{Namespace: "default", Name: "master"})
	now := time.Now()
	exp1 := now.Add(5 * time.Minute)

	// first user
	mstate.SetLease("mike", exp1, []data.HostID{0, 1, 2})
	usage := mstate.HostsUsage(now)
	assert.Equal(t, []int{1, 1, 1}, usage)
	expectedLeases := []data.Lease{
		data.Lease{User: "mike", Expiration: exp1, Hosts: []data.HostID{0, 1, 2}},
	}
	leases := mstate.Leases(now)
	assert.Equal(t, expectedLeases, leases)

	// still there
	now = now.Add(2 * time.Minute)
	usage = mstate.HostsUsage(now)
	assert.Equal(t, []int{1, 1, 1}, usage)
	expectedLeases = []data.Lease{
		data.Lease{User: "mike", Expiration: exp1, Hosts: []data.HostID{0, 1, 2}},
	}
	leases = mstate.Leases(now)
	assert.Equal(t, expectedLeases, leases)

	// expired
	now = exp1.Add(1 * time.Second)
	usage = mstate.HostsUsage(now)
	leases = mstate.Leases(now)
	assert.Equal(t, []int{}, usage)
	assert.Equal(t, 0, len(leases))
}

func TestStorageMultipleUsers(t *testing.T) {
	state := NewInMemoryState()
	mstate := state.TagState(data.Tag{Namespace: "default", Name: "master"})
	now := time.Now()
	exp1 := now.Add(5 * time.Minute)

	// first user
	mstate.SetLease("mike", exp1, []data.HostID{0, 1, 2})
	usage := mstate.HostsUsage(now)
	assert.Equal(t, []int{1, 1, 1}, usage)
	expectedLeases := []data.Lease{
		data.Lease{User: "mike", Expiration: exp1, Hosts: []data.HostID{0, 1, 2}},
	}
	leases := mstate.Leases(now)
	assert.Equal(t, expectedLeases, leases)

	// second user
	now = now.Add(2 * time.Minute)
	exp2 := now.Add(5 * time.Minute)
	mstate.SetLease("alice", exp2, []data.HostID{1, 2, 3})
	usage = mstate.HostsUsage(now)
	assert.Equal(t, []int{1, 2, 2, 1}, usage)
	expectedLeases = []data.Lease{
		data.Lease{User: "mike", Expiration: exp1, Hosts: []data.HostID{0, 1, 2}},
		data.Lease{User: "alice", Expiration: exp2, Hosts: []data.HostID{1, 2, 3}},
	}
	leases = mstate.Leases(now)
	assert.Equal(t, expectedLeases, leases)

	// one expired
	now = exp1.Add(1 * time.Second)
	usage = mstate.HostsUsage(now)
	assert.Equal(t, []int{0, 1, 1, 1}, usage)
	expectedLeases = []data.Lease{
		data.Lease{User: "alice", Expiration: exp2, Hosts: []data.HostID{1, 2, 3}},
	}
	leases = mstate.Leases(now)
	assert.Equal(t, expectedLeases, leases)

	// all gone
	now = exp2.Add(1 * time.Second)
	usage = mstate.HostsUsage(now)
	assert.Equal(t, []int{}, usage)
	leases = mstate.Leases(now)
	assert.Equal(t, []int{}, usage)
	assert.Equal(t, 0, len(leases))
}
