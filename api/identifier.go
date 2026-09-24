package api

import (
	"strconv"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
)

// ABOUT: Identifier is the input a Verifier receives. The Client builds one
// per identifier of a party: the tax_id and each entry of identities. It
// carries the normalized code and the position of the identifier in the
// party, so that a verifier can decide coverage and a Check can name the
// field it describes. Consumers never construct an Identifier: the public
// Client API takes GOBL types and does the walk.
//
// Every path in a report is an RFC 6901 JSON Pointer relative to the party
// document, for example "/tax_id" or "/identities/1/code". Consumers that
// match in Go use the typed values, Check.TaxID, Check.Identity and
// Mismatch.Field, rather than parsing paths. The pointer segments here are
// fixed field names and array indices, which never contain "/" or "~", so
// the builders do no escaping.

// Identifier is one identifier of a party as a verifier sees it.
type Identifier struct {
	// Path locates the identifier in the party: "/tax_id" or "/identities/N".
	Path string

	// Country is the tax country of the identifier.
	Country l10n.TaxCountryCode

	// Key is the org.Identity key, empty for a tax identity.
	Key cbc.Key

	// Type is the org.Identity type, empty for a tax identity.
	Type cbc.Code

	// Code is the normalized code of the identifier.
	Code cbc.Code
}

// IsTaxID reports whether the identifier is the party's tax identity.
func (id Identifier) IsTaxID() bool {
	return id.Path == PathTaxID
}

// PathTaxID is the path of a party's tax identity.
const PathTaxID = "/tax_id"

// PathName is the path of a party's name.
const PathName = "/name"

// IdentityPath returns the path of the identity at index i.
func IdentityPath(i int) string {
	return "/identities/" + strconv.Itoa(i)
}

// CodePath returns the path of the code field under an identifier path.
func CodePath(path string) string {
	return path + "/code"
}
