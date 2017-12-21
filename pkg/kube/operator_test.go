package kube

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestTransientError(t *testing.T) {
	// regular errors are not transient
	nonTransient := func() error {
		return errors.New("foo")
	}
	assert.False(t, IsTransient(nonTransient()))

	// transientError is transient
	transient := func() error {
		return transientError{errors.New("foo")}
	}
	assert.True(t, IsTransient(transient()))

	// wrapping a transient error is still a transient error
	wrapped := func() error {
		return errors.Wrap(transient(), "wrapped")
	}
	assert.True(t, IsTransient(wrapped()))
}
