package tin

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ABOUT: An invalid identifier is not represented here at all; it is a Check
// with StatusInvalid. A register that cannot answer is a Check with
// StatusUnverified that carries the error. Errors cover only the cases where
// the register could not give an answer. Callers match on the sentinels with
// errors.Is and on RateLimitedError with errors.As. Errors that come from an
// HTTP response carry the status via Code(), so callers can distinguish
// structurally.
//
// Verifiers map HTTP statuses as follows: 400 is ErrInput, 429 is
// RateLimitedError, and every other unexpected status, including edge
// responses such as 403, is ErrServer with the status as its code. ErrNetwork
// is reserved for failures below HTTP: dial, timeout, and unreadable bodies.

var (
	// ErrNetwork wraps transport failures: dial errors, timeouts, and
	// responses that could not be decoded. It says nothing about the identifier,
	// only that the request failed.
	ErrNetwork = NewError("network")

	// ErrServer is returned when the register answered with an unexpected
	// status. The HTTP status is available via Code().
	ErrServer = NewError("server")

	// ErrInput is returned when the input is malformed or incomplete: a
	// missing tax ID, an empty code, party data a register needs but the
	// request lacks, or a request the register rejected as badly formed.
	ErrInput = NewError("input")
)

// RateLimitedError reports that the register's request budget is exhausted,
// and how long to wait, so a caller can requeue with a precise delay instead
// of a blind backoff.
type RateLimitedError struct {
	RetryAfter time.Duration
}

// Error provides the string representation of the error.
func (e *RateLimitedError) Error() string {
	return "register: rate limited, retry in " + e.RetryAfter.String()
}

// Code returns the HTTP status that produces this error, which is always 429.
func (e *RateLimitedError) Code() string {
	return "429"
}

// Error contains the standard error definition for this domain.
type Error struct {
	key     string
	code    string
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

// WithCode adds a code to the error, usually the HTTP status.
func (e *Error) WithCode(code string) *Error {
	ne := e.copy()
	ne.code = code
	return ne
}

// Code provides the code of the error, usually the HTTP status, or an empty
// string when there is none.
func (e *Error) Code() string {
	return e.code
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
	out := []string{e.key}
	if e.code != "" {
		out = append(out, e.code)
	}
	if e.message != "" {
		out = append(out, e.message)
	}
	if e.cause != nil {
		out = append(out, e.cause.Error())
	}
	return strings.Join(out, ": ")
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
