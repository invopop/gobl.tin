package tin

import (
	"strings"

	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// identifier is the identifier of a party together with a normalized copy of
// the tax identity it comes from.
type identifier struct {
	Identifier
	taxID *tax.Identity
}

// walk lists the identifiers of a party: its tax identity, when it has a
// code after normalization. The party is left untouched; the tax identity is
// copied before normalization.
func walk(party *org.Party) []identifier {
	if party == nil || party.TaxID == nil {
		return nil
	}
	if id := fromTaxID(party.TaxID); id.Code != "" {
		return []identifier{id}
	}
	return nil
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
