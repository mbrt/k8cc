package errors

import (
	"testing"

	stderrs "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestTransientError(t *testing.T) {
	// regular errors are not transient
	nonTransient := func() error {
		return stderrs.New("foo")
	}
	assert.False(t, IsTransient(nonTransient()))

	// transientError is transient
	transient := func() error {
		return transientError{stderrs.New("foo")}
	}
	assert.True(t, IsTransient(transient()))

	// wrapping a transient error is still a transient error
	wrapped := func() error {
		return stderrs.Wrap(transient(), "wrapped")
	}
	assert.True(t, IsTransient(wrapped()))
}
