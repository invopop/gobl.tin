package tin

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
)

// ABOUT: Identifier describes the identifier of a party to a Verifier: its
// normalized tax ID code and where it sits in the party, so that a verifier
// can decide coverage and a Check can name the field it describes.
// Consumers never construct an Identifier: the public Client API takes GOBL
// types.

// Identifier is the identifier of a party as a verifier sees it.
type Identifier struct {
	// Path locates the identifier in the party: "/tax_id". Empty when a tax
	// ID is verified without a party document.
	Path string

	// Country is the tax country of the identifier.
	Country l10n.TaxCountryCode

	// Code is the normalized code of the identifier.
	Code cbc.Code
}

// PathTaxID is the path of a party's tax identity.
const PathTaxID = "/tax_id"

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
