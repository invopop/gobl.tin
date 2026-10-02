package tin

import (
	"strings"
	"unicode"

	"github.com/invopop/gobl/org"
)

// ABOUT: Mismatches are computed here and not in the verifiers, because a
// verifier sees one identifier and the comparison needs the whole party. A
// mismatch never changes the party; it names the field, the value in the
// party and the value in the record.

// mismatches compares the party with the record behind a check. The party
// may be nil when a single tax ID is verified; the tax identity echo is then
// the only comparison.
func mismatches(party *org.Party, id identifier, check *Check) []*Mismatch {
	var out []*Mismatch
	if m := taxIDMismatch(id, check); m != nil {
		out = append(out, m)
	}
	if party == nil || check.Record == nil {
		return out
	}
	if m := nameMismatch(party, check.Record); m != nil {
		out = append(out, m)
	}
	return out
}

// taxIDMismatch compares the tax identity the register echoes with the
// party's normalized tax identity. A different code points at /tax_id/code
// and a different country at /tax_id/country; for a tax ID verified without
// a party, the pointers are relative to it: /code and /country.
func taxIDMismatch(id identifier, check *Check) *Mismatch {
	if check.TaxID == nil || check.TaxID == id.taxID {
		return nil
	}
	echo := normalizeTaxID(check.TaxID)
	switch {
	case echo.Country != id.taxID.Country:
		return &Mismatch{
			Field:    MismatchTaxID,
			Path:     CountryPath(id.Path),
			Document: id.taxID.Country.String(),
			Register: echo.Country.String(),
		}
	case echo.Code != id.taxID.Code:
		return &Mismatch{
			Field:    MismatchTaxID,
			Path:     CodePath(id.Path),
			Document: id.taxID.Code.String(),
			Register: echo.Code.String(),
		}
	default:
		return nil
	}
}

// nameMismatch compares the party's name with the record's when both are
// set. A record name with no letter or digit, such as a "---" placeholder,
// is no name.
func nameMismatch(party *org.Party, rec *Record) *Mismatch {
	name := strings.TrimSpace(rec.Name)
	if party.Name == "" || !hasLetterOrDigit(name) || NameMatches(name, party.Name) {
		return nil
	}
	return &Mismatch{Field: MismatchName, Path: PathName, Document: party.Name, Register: name}
}

// hasLetterOrDigit reports whether s holds at least one letter or digit.
func hasLetterOrDigit(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
