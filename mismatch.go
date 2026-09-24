package tin

import (
	"github.com/invopop/gobl.tin/api"
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
func mismatches(party *org.Party, ids []identifier, id identifier, check *api.Check) []*api.Mismatch {
	var out []*api.Mismatch
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
// party's normalized tax identity.
func taxIDMismatch(id identifier, check *api.Check) *api.Mismatch {
	if !id.IsTaxID() || check.TaxID == nil || check.TaxID == id.taxID {
		return nil
	}
	echo := normalizeTaxID(check.TaxID)
	if echo.Country == id.taxID.Country && echo.Code == id.taxID.Code {
		return nil
	}
	return &api.Mismatch{
		Field:    api.MismatchTaxID,
		Path:     api.CodePath(api.PathTaxID),
		Document: id.taxID.String(),
		Register: echo.String(),
	}
}

// nameMismatch compares the party's name with the record's when both are
// set.
func nameMismatch(party *org.Party, rec *api.Record) *api.Mismatch {
	if party.Name == "" || rec.Name == "" || api.NameMatches(rec.Name, party.Name) {
		return nil
	}
	return &api.Mismatch{Field: api.MismatchName, Path: api.PathName, Document: party.Name, Register: rec.Name}
}

// identityMismatches compares each record identity with the party
// identities of the same country and type.
func identityMismatches(ids []identifier, rec *api.Record) []*api.Mismatch {
	var out []*api.Mismatch
	for _, r := range rec.Identities {
		if r == nil || r.Type == "" {
			continue
		}
		code := normalizeCode(r.Code)
		for _, id := range ids {
			if !sameIdentityType(id, r) || id.Code == code {
				continue
			}
			out = append(out, &api.Mismatch{
				Field:    api.MismatchIdentity,
				Path:     api.CodePath(id.Path),
				Document: id.Code.String(),
				Register: code.String(),
			})
		}
	}
	return out
}

// sameIdentityType reports whether the identifier is an identity of the same
// country and type as the record identity.
func sameIdentityType(id identifier, r *org.Identity) bool {
	return id.identity != nil && id.identity.Country == r.Country && id.Type == r.Type
}
