package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mbrt/k8cc/pkg/data"
)

func TestStorageSingleUser(t *testing.T) {
	state := NewInMemoryState()
	mstate := state.TagState("master")
	now := time.Now()
	exp1 := now.Add(5 * time.Minute)

	// first user
	mstate.SetLease("mike", exp1, []data.HostID{0, 1, 2})
	usage := mstate.HostsUsage(now)
	assert.Equal(t, []int{1, 1, 1}, usage)

	// still there
	now = now.Add(2 * time.Minute)
	usage = mstate.HostsUsage(now)
	assert.Equal(t, []int{1, 1, 1}, usage)

	// expired
	now = exp1.Add(1 * time.Second)
	usage = mstate.HostsUsage(now)
	assert.Equal(t, []int{}, usage)
}

func TestStorageMultipleUsers(t *testing.T) {
	state := NewInMemoryState()
	mstate := state.TagState("master")
	now := time.Now()
	exp1 := now.Add(5 * time.Minute)

	// first user
	mstate.SetLease("mike", exp1, []data.HostID{0, 1, 2})
	usage := mstate.HostsUsage(now)
	assert.Equal(t, []int{1, 1, 1}, usage)

	// second user
	now = now.Add(2 * time.Minute)
	exp2 := now.Add(5 * time.Minute)
	mstate.SetLease("alice", exp2, []data.HostID{1, 2, 3})
	usage = mstate.HostsUsage(now)
	assert.Equal(t, []int{1, 2, 2, 1}, usage)

	// one expired
	now = exp1.Add(1 * time.Second)
	usage = mstate.HostsUsage(now)
	assert.Equal(t, []int{0, 1, 1, 1}, usage)

	// all gone
	now = exp2.Add(1 * time.Second)
	usage = mstate.HostsUsage(now)
	assert.Equal(t, []int{}, usage)
}
