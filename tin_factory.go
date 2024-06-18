package gobltin

// Factory for getting all validation functions
func GetTinLookup(countryCode string) TinLookup {
	switch {
	case isEuropeanCountryCode(countryCode): // For the moment it only supports VIES lookup
		return VIESLookup{}
	// Add cases for other countries and their specific validators
	default:
		return nil // nil in case we don't have a validator for the country code
	}
}

// List of all EU country codes supported by VIES
var europeanCountryCodes = []string{
	"AT", "BE", "BG", "CY", "CZ", "DE", "DK", "EE", "EL", "ES", "FI",
	"FR", "HR", "HU", "IE", "IT", "LT", "LU", "LV", "MT", "NL", "PL",
	"PT", "RO", "SE", "SI", "SK", "XI",
}

func isEuropeanCountryCode(code string) bool {
	for _, c := range europeanCountryCodes {
		if c == code {
			return true
		}
	}
	return false
}
