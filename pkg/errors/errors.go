package errors

import stderrs "github.com/pkg/errors"

// IsTransient returns true if an error is transient
func IsTransient(err error) bool {
	te, ok := stderrs.Cause(err).(transient)
	return ok && te.Transient()
}

// TransientError wraps the given error and makes it into a transient one
func TransientError(err error) error {
	return transientError{err}
}

type transient interface {
	Transient() bool
}

type transientError struct {
	error
}

func (e transientError) Error() string   { return e.error.Error() }
func (e transientError) Transient() bool { return true }
