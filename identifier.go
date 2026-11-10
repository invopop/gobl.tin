package tin

import (
	"strings"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// identifier is the identifier of a party together with a normalized copy of
// the tax identity or the identity it comes from.
type identifier struct {
	Identifier
	taxID    *tax.Identity
	identity *org.Identity
}

// walk lists the identifiers of a party: its tax identity, then each identity
// with a type, when it has a code after normalization. The party is left
// untouched; each identifier is copied before normalization.
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
	for i, ident := range party.Identities {
		if ident == nil {
			continue
		}
		if id := fromIdentity(i, ident); id.Type != "" && id.Code != "" {
			out = append(out, id)
		}
	}
	return out
}

// fromIdentity copies and normalizes the identity at the index.
func fromIdentity(i int, ident *org.Identity) identifier {
	cp := normalizeIdentity(ident)
	return identifier{
		Identifier: Identifier{
			Path:    IdentityPath(i),
			Type:    cp.Type,
			Country: l10n.TaxCountryCode(cp.Country),
			Code:    cp.Code,
		},
		identity: cp,
	}
}

// normalizeIdentity returns a copy of the identity with the type and the
// country upper cased, and the code stripped of separators.
func normalizeIdentity(ident *org.Identity) *org.Identity {
	cp := *ident
	cp.Type = cbc.Code(strings.ToUpper(strings.TrimSpace(cp.Type.String())))
	cp.Country = l10n.ISOCountryCode(strings.ToUpper(strings.TrimSpace(cp.Country.String())))
	cp.Code = cbc.NormalizeAlphanumericalCode(cp.Code)
	return &cp
}

// sameIdentity reports whether the party holds the identity, by type and
// code once normalized.
func sameIdentity(ident *org.Identity, party *org.Party) bool {
	if ident == nil {
		return false
	}
	for _, p := range party.Identities {
		if p == nil {
			continue
		}
		if cp := normalizeIdentity(p); cp.Type == ident.Type && cp.Code == ident.Code {
			return true
		}
	}
	return false
}

// fromTaxID copies and normalizes a tax identity into an identifier.
func fromTaxID(tid *tax.Identity) identifier {
	cp := normalizeTaxID(tid)
	return identifier{
		Identifier: Identifier{
			Path:    PathTaxID,
			Country: cp.Country,
			Code:    cp.Code,
		},
		taxID: cp,
	}
}

// normalizeTaxID returns a copy of the tax identity with the country and
// the code upper cased, and the code stripped of separators and of a
// leading country prefix.
func normalizeTaxID(tid *tax.Identity) *tax.Identity {
	cp := *tid
	cp.Country = l10n.TaxCountryCode(strings.ToUpper(strings.TrimSpace(cp.Country.String())))
	tax.NormalizeIdentity(&cp)
	return &cp
}
