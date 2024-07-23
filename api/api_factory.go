package api

import (
	"github.com/invopop/gobl.tin/api/vies"
	"github.com/invopop/gobl/l10n"
)

// GetLookupAPI returns the TinLookup for a given country code
func GetLookupAPI(countryCode l10n.CountryCode) LookupAPI {
	switch {
	case isEuropeanCountryCode(countryCode): // For the moment it only supports VIES lookup
		return vies.API{}
	// Add cases for other countries and their specific validators
	default:
		return nil // nil in case we don't have a validator for the country code
	}
}

// List of all EU country codes supported by VIES
// XI is used for Northern Ireland when needs to be distinguished from UK
// EL is not the official country code for Greece, but it is used by VIES
var europeanCountryCodes = []l10n.CountryCode{
	l10n.AT, l10n.BE, l10n.BG, l10n.CY, l10n.CZ, l10n.DE, l10n.DK, l10n.EE, "EL", l10n.ES,
	l10n.FI, l10n.FR, l10n.HR, l10n.HU, l10n.IE, l10n.IT, l10n.LT, l10n.LU, l10n.LV, l10n.MT,
	l10n.NL, l10n.PL, l10n.PT, l10n.RO, l10n.SE, l10n.SI, l10n.SK, "XI",
}

func isEuropeanCountryCode(code l10n.CountryCode) bool {
	for _, c := range europeanCountryCodes {
		if c == code {
			return true
		}
	}
	return false
}
