package api

import (
	"errors"
	"fmt"
	"time"
)

// ABOUT: An invalid TIN is not represented here at all; it is a Result with
// Valid false. Errors cover only the cases where the registry could not give
// an answer. Callers match on the sentinels with errors.Is and on
// RateLimitedError with errors.As.

var (
	// ErrNotSupported is returned when no registry covers the country.
	ErrNotSupported = NewError("not-supported")

	// ErrNetwork wraps transport failures and registry server errors. It does
	// not say anything about the TIN, only that the request failed.
	ErrNetwork = NewError("network")

	// ErrInput is returned when the input is malformed or incomplete: a
	// missing tax ID, an empty code, or a request the registry rejected as
	// badly formed.
	ErrInput = NewError("input")
)

// RateLimitedError reports that the registry's request budget is exhausted,
// and how long to wait, so a caller can requeue with a precise delay instead
// of a blind backoff.
type RateLimitedError struct {
	RetryAfter time.Duration
}

// Error provides the string representation of the error.
func (e *RateLimitedError) Error() string {
	return "registry: rate limited, retry in " + e.RetryAfter.String()
}

// Error contains the standard error definition for this domain.
type Error struct {
	key     string
	cause   error
	message string
}

// NewError instantiates a new error with the given key.
func NewError(key string) *Error {
	return &Error{key: key}
}

func (e *Error) copy() *Error {
	ne := new(Error)
	*ne = *e
	return ne
}

// WithCause attaches an underlying error, preserving it for errors.Is and
// errors.As via Unwrap.
func (e *Error) WithCause(cause error) *Error {
	ne := e.copy()
	ne.cause = cause
	return ne
}

// WithMessage adds a message to the Error.
func (e *Error) WithMessage(message string) *Error {
	ne := e.copy()
	ne.message = message
	return ne
}

// WithMsgf adds a message with formatting details.
func (e *Error) WithMsgf(message string, args ...any) *Error {
	return e.WithMessage(fmt.Sprintf(message, args...))
}

// Error provides the string representation of the error.
func (e *Error) Error() string {
	if e.message == "" {
		if e.cause == nil {
			return e.key
		}
		return fmt.Sprintf("%s: %s", e.key, e.cause.Error())
	}
	if e.cause == nil {
		return fmt.Sprintf("%s: %s", e.key, e.message)
	}
	return fmt.Sprintf("%s: %s (%s)", e.key, e.message, e.cause.Error())
}

// Is checks to see if the target error matches the current error or part of
// the chain.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return errors.Is(e.cause, target)
	}
	return e.key == t.key
}

// Unwrap provides the underlying error, if any.
func (e *Error) Unwrap() error {
	return e.cause
}

// Message returns just the message component, if present.
func (e *Error) Message() string {
	return e.message
}
