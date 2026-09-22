package tin

import (
	"github.com/invopop/gobl.tin/api"
	"github.com/invopop/gobl.tin/api/vies"
	"github.com/invopop/gobl/l10n"
)

// lookupAPIFor returns the registry client for a country, or nil when no
// registry covers it.
func lookupAPIFor(countryCode l10n.TaxCountryCode) api.LookupAPI {
	switch {
	case isEuropeanCountryCode(countryCode): // For the moment it only supports VIES lookup
		return vies.New()
	// Add cases for other countries and their specific registries.
	default:
		return nil
	}
}

// List of all EU country codes supported by VIES.
// XI is used for Northern Ireland when it needs to be distinguished from GB.
// EL is not the official country code for Greece, but it is used by VIES.
var europeanCountryCodes = []l10n.Code{
	l10n.AT, l10n.BE, l10n.BG, l10n.CY, l10n.CZ, l10n.DE, l10n.DK, l10n.EE, l10n.EL, l10n.ES,
	l10n.FI, l10n.FR, l10n.HR, l10n.HU, l10n.IE, l10n.IT, l10n.LT, l10n.LU, l10n.LV, l10n.MT,
	l10n.NL, l10n.PL, l10n.PT, l10n.RO, l10n.SE, l10n.SI, l10n.SK, l10n.XI,
}

func isEuropeanCountryCode(code l10n.TaxCountryCode) bool {
	return code.Code().In(europeanCountryCodes...)
}
