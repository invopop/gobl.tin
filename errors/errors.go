// Package errors provides a way to handle different errors in gobl.tin
package errors

import (
	"errors"
	"fmt"
)

// Error contains the standard error definition for this domain.
type Error struct {
	errorType string
	cause     error
	message   string
}

var (
	// ErrNotSupported is used when the country is not supported
	ErrNotSupported = NewError("Country not supported")

	// ErrNetwork is an error that appears when there is a network issue
	ErrNetwork = NewError("network")

	// ErrInput is an error that appears when the input is invalid
	ErrInput = NewError("input")
)

func (e *Error) copy() *Error {
	ne := new(Error)
	*ne = *e
	return ne
}

// NewError instantiates a new error.
func NewError(errorType string) *Error {
	return &Error{errorType: errorType}
}

// WithCause attaches any error instance to the Error.
func (e *Error) WithCause(cause error) *Error {
	ne := e.copy()
	ne.cause = cause
	return ne
}

// Error provides the string representation of the error.
func (e *Error) Error() string {
	if e.message == "" {
		if e.cause == nil {
			return e.errorType
		}
		return e.cause.Error()
	}
	if e.cause == nil {
		return e.message
	}
	return fmt.Sprintf("%s (%s)", e.message, e.cause.Error())
}

// WithMessage adds a message to the Error.
func (e *Error) WithMessage(message string) *Error {
	ne := e.copy()
	ne.message = message
	return ne
}

// Is checks to see if the target error matches the current error or
// part of the chain.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return errors.Is(e.cause, target)
	}
	return e.errorType == t.errorType
}

// Cause returns the error that caused this error.
func (e *Error) Cause() error {
	return e.cause
}

// Message returns just the message component, if present
func (e *Error) Message() string {
	return e.message
}
