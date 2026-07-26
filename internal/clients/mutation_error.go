package clients

import (
	"errors"
	"net/http"
)

// MutationDisposition describes what a provider response proves about a write.
type MutationDisposition uint8

const (
	MutationDispositionUnknown MutationDisposition = iota
	MutationDispositionAuthentication
	MutationDispositionConflict
	MutationDispositionRejected
	MutationDispositionAmbiguous
)

type mutationError struct {
	disposition MutationDisposition
	cause       error
}

func (e *mutationError) Error() string {
	return e.cause.Error()
}

func (e *mutationError) Unwrap() error {
	return e.cause
}

func newMutationError(disposition MutationDisposition, cause error) error {
	return &mutationError{disposition: disposition, cause: cause}
}

func mutationDispositionForStatus(status int) MutationDisposition {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return MutationDispositionAuthentication
	case status == http.StatusConflict:
		return MutationDispositionConflict
	case status >= 400 && status < 500:
		return MutationDispositionRejected
	default:
		return MutationDispositionAmbiguous
	}
}

// MutationDispositionOf returns the provider-level proof attached to a failed
// mutation.
func MutationDispositionOf(err error) MutationDisposition {
	var mutationErr *mutationError
	if errors.As(err, &mutationErr) {
		return mutationErr.disposition
	}
	return MutationDispositionUnknown
}
