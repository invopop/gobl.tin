package tin

import "github.com/invopop/gobl.tin/api"

// ABOUT: The error taxonomy lives in the api package so that verifiers can
// return it without an import cycle. These aliases keep the root package
// self-sufficient for consumers.
//
// An invalid identifier is not an error. It is a Check with StatusInvalid. A
// register that cannot answer is a Check with StatusUnverified that carries
// the error.

// Error is the structured error type used across the library.
type Error = api.Error

// RateLimitedError reports that a register's request budget is exhausted and
// how long to wait before retrying. Match it with errors.As.
type RateLimitedError = api.RateLimitedError

var (
	// ErrNotSupported is reserved for callers that need to reject an
	// identifier no verifier covers. The Client reports StatusUnsupported.
	ErrNotSupported = api.ErrNotSupported

	// ErrNetwork wraps transport failures: dial errors, timeouts, and
	// responses that could not be decoded.
	ErrNetwork = api.ErrNetwork

	// ErrServer is returned when the register answered with an unexpected
	// status. The HTTP status is available via Code().
	ErrServer = api.ErrServer

	// ErrInput is returned when the input is malformed or incomplete.
	ErrInput = api.ErrInput
)
