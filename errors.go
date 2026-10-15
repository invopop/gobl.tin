package tin

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ABOUT: errors are for a register that cannot answer; an invalid identifier
// is a Check with StatusInvalid. HTTP statuses map as: 400 is ErrInput, 429 is
// RateLimitedError, any other is ErrServer with the status as code.

var (
	// ErrNetwork is a transport failure: dial, timeout or unreadable response.
	ErrNetwork = NewError("network")

	// ErrServer is an unexpected register answer, with the HTTP status as
	// code, or a configuration error, without a code.
	ErrServer = NewError("server")

	// ErrInput is malformed or incomplete input. Without a code, it comes
	// before any request and its message says what the input lacks.
	ErrInput = NewError("input")
)

// RateLimitedError reports a rate limit and how long to wait.
type RateLimitedError struct {
	RetryAfter time.Duration
	// Reason is the register's name for the limit, such as
	// MS_MAX_CONCURRENT_REQ.
	Reason string
}

// Error provides the string representation of the error.
func (e *RateLimitedError) Error() string {
	if e.Reason != "" {
		return "register: rate limited (" + e.Reason + "), retry in " + e.RetryAfter.String()
	}
	return "register: rate limited, retry in " + e.RetryAfter.String()
}

// Code returns 429.
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

// WithCause attaches an underlying error.
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

// Code returns the code of the error, usually the HTTP status.
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

// Is matches errors by key, or the cause.
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
