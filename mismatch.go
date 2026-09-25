package tin

import (
	"github.com/invopop/gobl/org"
)

// ABOUT: Mismatches are computed here and not in the verifiers, because a
// verifier sees one identifier and the comparison needs the whole party. A
// mismatch never changes the party; it names the field, the value in the
// party and the value in the record, so that a Policy can decide what to
// write.

// mismatches compares the party with the record behind a check. The party
// and its identifiers may be nil when a single identifier is verified; the
// tax identity echo is then the only comparison.
func mismatches(party *org.Party, ids []identifier, id identifier, check *Check) []*Mismatch {
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
	out = append(out, identityMismatches(ids, check.Record)...)
	return out
}

// taxIDMismatch compares the tax identity the register echoes with the
// party's normalized tax identity. A different code points at /tax_id/code;
// a different country at /tax_id/country.
func taxIDMismatch(id identifier, check *Check) *Mismatch {
	if !id.IsTaxID() || check.TaxID == nil || check.TaxID == id.taxID {
		return nil
	}
	echo := normalizeTaxID(check.TaxID)
	switch {
	case echo.Country != id.taxID.Country:
		return &Mismatch{
			Field:    MismatchTaxID,
			Path:     CountryPath(PathTaxID),
			Document: id.taxID.Country.String(),
			Register: echo.Country.String(),
		}
	case echo.Code != id.taxID.Code:
		return &Mismatch{
			Field:    MismatchTaxID,
			Path:     CodePath(PathTaxID),
			Document: id.taxID.Code.String(),
			Register: echo.Code.String(),
		}
	default:
		return nil
	}
}

// nameMismatch compares the party's name with the record's when both are
// set.
func nameMismatch(party *org.Party, rec *Record) *Mismatch {
	if party.Name == "" || rec.Name == "" || NameMatches(rec.Name, party.Name) {
		return nil
	}
	return &Mismatch{Field: MismatchName, Path: PathName, Document: party.Name, Register: rec.Name}
}

// identityMismatches compares each record identity with the party
// identities of the same kind. A record identity with neither type nor key
// cannot be matched and is skipped.
func identityMismatches(ids []identifier, rec *Record) []*Mismatch {
	var out []*Mismatch
	for _, r := range rec.Identities {
		if r == nil || kindless(r) {
			continue
		}
		code := normalizeCode(r.Code)
		for _, id := range ids {
			if !sameKind(id.identity, r) || id.Code == code {
				continue
			}
			out = append(out, &Mismatch{
				Field:    MismatchIdentity,
				Path:     CodePath(id.Path),
				Document: id.Code.String(),
				Register: code.String(),
			})
		}
	}
	return out
}
