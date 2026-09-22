// Package api defines the contract that TIN registry clients implement, the
// Result they return, and the error taxonomy they use.
//
// ABOUT: This package sits below the registry implementations so that both
// they and the root package can share these types without an import cycle.
// The root package aliases the user-facing types, so consumers normally only
// import github.com/invopop/gobl.tin.
package api

import (
	"context"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
)

// Result is the outcome of a registry lookup.
//
// An invalid TIN is an ordinary outcome, never an error: it is reported as a
// Result with Valid false and a nil error. Errors are reserved for cases where
// the registry could not answer.
type Result struct {
	// Valid reports whether the registry recognises the TIN.
	Valid bool

	// Name is the name the registry holds for the party. Empty when the
	// registry masks it, which some member states always do.
	Name string

	// Address is the registered address as a single unstructured string, in
	// whatever layout the registry uses. Empty when the registry masks it.
	Address string

	// Source identifies the registry that answered, for example "vies".
	Source cbc.Key
}

// LookupAPI is the interface that TIN registry clients implement.
type LookupAPI interface {
	// LookupTIN checks the tax identity against the registry. An unknown or
	// malformed-per-registry number yields Valid false, not an error.
	LookupTIN(ctx context.Context, tid *tax.Identity) (*Result, error)
}
