package service

import (
	"errors"
	"fmt"
)

// Error kinds. Every error returned by this package that is caused by the
// caller (rather than by an infrastructure failure) wraps exactly one of
// these, so transports can map them with errors.Is.
var (
	ErrInvalid      = errors.New("invalid input")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)

// Error is a caller-facing error: Msg is safe to show to users and Kind is
// one of the Err* sentinels above.
type Error struct {
	Kind error
	Msg  string
}

// Error implements the error interface and returns the human-readable message.
func (e *Error) Error() string { return e.Msg }

// Unwrap returns the error kind so errors.Is(err, ErrNotFound) etc. work.
func (e *Error) Unwrap() error { return e.Kind }

func newError(kind error, format string, args ...any) error {
	return &Error{Kind: kind, Msg: fmt.Sprintf(format, args...)}
}
