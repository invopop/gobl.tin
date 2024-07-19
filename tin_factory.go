package gobltin

import "github.com/invopop/gobl/l10n"

// GetTinLookup returns the TinLookup for a given country code
func GetTinLookup(countryCode l10n.CountryCode) TinLookup {
	switch {
	case isEuropeanCountryCode(countryCode): // For the moment it only supports VIES lookup
		return VIESLookup{}
	// Add cases for other countries and their specific validators
	default:
		return nil // nil in case we don't have a validator for the country code
	}
}

// List of all EU country codes supported by VIES
// XI is used for Northern Ireland when needs to be distinguished from UK
var europeanCountryCodes = []l10n.CountryCode{
	"AT", "BE", "BG", "CY", "CZ", "DE", "DK", "EE", "EL", "ES", "FI",
	"FR", "HR", "HU", "IE", "IT", "LT", "LU", "LV", "MT", "NL", "PL",
	"PT", "RO", "SE", "SI", "SK", "XI",
}

func isEuropeanCountryCode(code l10n.CountryCode) bool {
	for _, c := range europeanCountryCodes {
		if c == code {
			return true
		}
	}
	return false
}
