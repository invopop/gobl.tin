package tin

import (
	"strconv"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
)

// ABOUT: Identifier describes the identifier of a party to a Verifier: its
// normalized code, its type for an entry of identities, and where it sits in
// the party, so that a verifier can decide coverage and a Check can name the
// field it describes.
// Consumers never construct an Identifier: the public Client API takes GOBL
// types.

// Identifier is the identifier of a party as a verifier sees it.
type Identifier struct {
	// Path locates the identifier in the party: "/tax_id" or
	// "/identities/0". Empty when a tax ID is verified without a party
	// document.
	Path string

	// Type is the type of an identity, such as "SIREN", and empty for a tax
	// ID.
	Type cbc.Code

	// Country is the tax country of the identifier. An identity without a
	// country has none.
	Country l10n.TaxCountryCode

	// Code is the normalized code of the identifier.
	Code cbc.Code
}

// PathTaxID is the path of a party's tax identity.
const PathTaxID = "/tax_id"

// IdentityPath returns the path of the party's identity at the index.
func IdentityPath(i int) string {
	return "/identities/" + strconv.Itoa(i)
}

// PathName is the path of a party's name.
const PathName = "/name"

// CodePath returns the path of the code field under an identifier path.
func CodePath(path string) string {
	return path + "/code"
}

// CountryPath returns the path of the country field under an identifier
// path.
func CountryPath(path string) string {
	return path + "/country"
}
