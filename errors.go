package tin

import "github.com/invopop/gobl.tin/api"

// ABOUT: The error taxonomy lives in the api package so that registry clients
// can return it without an import cycle. These aliases keep the root package
// self-sufficient for consumers.
//
// An invalid TIN is not an error. It is a Result with Valid false.

// Error is the structured error type used across the library.
type Error = api.Error

// RateLimitedError reports that a registry's request budget is exhausted and
// how long to wait before retrying. Match it with errors.As.
type RateLimitedError = api.RateLimitedError

var (
	// ErrNotSupported is returned when no registry covers the country.
	ErrNotSupported = api.ErrNotSupported

	// ErrNetwork wraps transport failures and registry server errors.
	ErrNetwork = api.ErrNetwork

	// ErrInput is returned when the input is malformed or incomplete.
	ErrInput = api.ErrInput
)
