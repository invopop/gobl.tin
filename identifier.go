package tin

import (
	"fmt"
	"strings"

	"github.com/invopop/gobl.tin/api"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// identifier is one identifier of a party together with a normalized copy of
// the GOBL value it comes from. Exactly one of taxID and identity is set.
type identifier struct {
	api.Identifier
	taxID    *tax.Identity
	identity *org.Identity
}

// walk lists the identifiers of a party in document order: the tax identity
// first, then each identity. The party is left untouched; every GOBL value
// is copied before normalization. A tax identity without a code and a nil
// identity are skipped.
func walk(party *org.Party) []identifier {
	if party == nil {
		return nil
	}
	var out []identifier
	if party.TaxID != nil && party.TaxID.Code != "" {
		out = append(out, fromTaxID(party.TaxID))
	}
	for i, id := range party.Identities {
		if id == nil {
			continue
		}
		out = append(out, fromIdentity(i, id))
	}
	return out
}

// fromTaxID copies and normalizes a tax identity into an identifier.
func fromTaxID(tid *tax.Identity) identifier {
	cp := normalizeTaxID(tid)
	return identifier{
		Identifier: api.Identifier{
			Path:    api.PathTaxID,
			Country: cp.Country,
			Code:    cp.Code,
		},
		taxID: cp,
	}
}

// fromIdentity copies and normalizes the identity at index i into an
// identifier.
func fromIdentity(i int, id *org.Identity) identifier {
	cp := normalizeIdentity(id)
	return identifier{
		Identifier: api.Identifier{
			Path:    fmt.Sprintf("identities[%d]", i),
			Country: l10n.TaxCountryCode(cp.Country),
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
