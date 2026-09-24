package tin

import (
	"strings"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// identifier is one identifier of a party together with a normalized copy of
// the GOBL value it comes from. Exactly one of taxID and identity is set.
type identifier struct {
	Identifier
	taxID    *tax.Identity
	identity *org.Identity
}

// walk lists the identifiers of a party in document order: the tax identity
// first, then each identity. The party is left untouched; every GOBL value
// is copied before normalization. A nil identity, and an identifier whose
// code is empty after normalization, are skipped: there is nothing to ask a
// register about.
func walk(party *org.Party) []identifier {
	if party == nil {
		return nil
	}
	var out []identifier
	if party.TaxID != nil {
		if id := fromTaxID(party.TaxID); id.Code != "" {
			out = append(out, id)
		}
	}
	for i, oid := range party.Identities {
		if oid == nil {
			continue
		}
		if id := fromIdentity(i, oid); id.Code != "" {
			out = append(out, id)
		}
	}
	return out
}

// fromTaxID copies and normalizes a tax identity into an identifier.
func fromTaxID(tid *tax.Identity) identifier {
	cp := normalizeTaxID(tid)
	return identifier{
		Identifier: Identifier{
			Path:    PathTaxID,
			TaxID:   true,
			Country: cp.Country,
			Code:    cp.Code,
		},
		taxID: cp,
	}
}

// fromIdentity copies and normalizes the identity at index i into an
// identifier. The ISO country becomes the tax country by cast: an identity
// of Greece stays GR, while VIES knows Greece as EL.
func fromIdentity(i int, id *org.Identity) identifier {
	cp := normalizeIdentity(id)
	return identifier{
		Identifier: Identifier{
			Path:    IdentityPath(i),
			Country: cp.Country.Code().Tax(),
			Key:     cp.Key,
			Type:    cp.Type,
			Code:    cp.Code,
		},
		identity: cp,
	}
}

// normalizeTaxID returns a copy of the tax identity with the code upper
// cased, stripped of separators and of a leading country prefix.
func normalizeTaxID(tid *tax.Identity) *tax.Identity {
	cp := *tid
	tax.NormalizeIdentity(&cp)
	return &cp
}

// normalizeIdentity returns a copy of the identity with the code trimmed and
// upper cased.
func normalizeIdentity(id *org.Identity) *org.Identity {
	cp := *id
	cp.Code = normalizeCode(cp.Code)
	return &cp
}

// normalizeCode trims and upper cases an identity code.
func normalizeCode(c cbc.Code) cbc.Code {
	return cbc.Code(strings.ToUpper(strings.TrimSpace(c.String())))
}

// sameKind reports whether two identities are of one kind: same country,
// type and key. The code is not compared.
func sameKind(a, b *org.Identity) bool {
	return a != nil && b != nil && a.Country == b.Country && a.Type == b.Type && a.Key == b.Key
}

// kindless reports whether an identity has neither a type nor a key, so that
// it cannot be matched against the party's identities.
func kindless(id *org.Identity) bool {
	return id.Type == "" && id.Key == ""
}
