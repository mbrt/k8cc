package data

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStorageSingleUser(t *testing.T) {
	storage := NewInMemoryStorage()
	now := time.Now()
	exp1 := now.Add(5 * time.Minute)

	// first user
	storage.SetLease("master", "mike", exp1, []BuildHostID{0, 1, 2})
	usage := storage.Usage("master", now)
	assert.Equal(t, []int{1, 1, 1}, usage)

	// still there
	now = now.Add(2 * time.Minute)
	usage = storage.Usage("master", now)
	assert.Equal(t, []int{1, 1, 1}, usage)

	// expired
	now = exp1.Add(1 * time.Second)
	usage = storage.Usage("master", now)
	assert.Equal(t, []int{}, usage)
}

func TestStorageMultipleUsers(t *testing.T) {
	storage := NewInMemoryStorage()
	now := time.Now()
	exp1 := now.Add(5 * time.Minute)

	// first user
	storage.SetLease("master", "mike", exp1, []BuildHostID{0, 1, 2})
	usage := storage.Usage("master", now)
	assert.Equal(t, []int{1, 1, 1}, usage)

	// second user
	now = now.Add(2 * time.Minute)
	exp2 := now.Add(5 * time.Minute)
	storage.SetLease("master", "alice", exp2, []BuildHostID{1, 2, 3})
	usage = storage.Usage("master", now)
	assert.Equal(t, []int{1, 2, 2, 1}, usage)

	// one expired
	now = exp1.Add(1 * time.Second)
	usage = storage.Usage("master", now)
	assert.Equal(t, []int{0, 1, 1, 1}, usage)

	// all gone
	now = exp2.Add(1 * time.Second)
	usage = storage.Usage("master", now)
	assert.Equal(t, []int{}, usage)
}
