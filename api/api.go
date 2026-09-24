// Package api defines the Verifier port that register clients implement, the
// report types they fill, and the error taxonomy they use.
//
// ABOUT: This package sits below the registry implementations so that both
// they and the root package can share these types without an import cycle.
// The root package aliases the user-facing types, so consumers normally only
// import github.com/invopop/gobl.tin.
package api

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// Result is the outcome of a registry lookup, expressed in GOBL types so
// that consumers can store and apply it without re-mapping.
//
// An invalid TIN is an ordinary outcome, never an error: it is reported as a
// Result with Valid false and a nil error. Errors are reserved for cases where
// the registry could not answer.
type Result struct {
	// Valid reports whether the registry recognises the TIN.
	Valid bool

	// Source identifies the registry that answered, for example "vies".
	Source cbc.Key

	// TaxID is the tax identity as the registry confirmed it.
	TaxID *tax.Identity

	// Name is the name the registry holds for the party. Empty when the
	// registry masks it, which some member states always do.
	Name string

	// Identities carries registry identifiers that are not tax IDs, such as
	// a company registration number. Registries that only answer about tax
	// identities leave it empty.
	Identities []*org.Identity

	// Address is the registered address, when the registry provides it in
	// structured form. VIES returns only an unstructured string, so it
	// leaves this nil.
	Address *org.Address
}
