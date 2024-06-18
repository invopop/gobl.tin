package main

// Factory for getting all validation functions
func GetTinLookup(countryCode string) TinLookup {
	switch {
	case isEuropeanCountryCode(countryCode): // List all EU country codes here
		return VIESLookup{}
	// Add cases for other countries and their specific validators
	default:
		return nil // or a default validator if applicable
	}
}

// List of all EU country codes
var europeanCountryCodes = []string{
	"AT", "BE", "BG", "CY", "CZ", "DE", "DK", "EE", "EL", "ES", "FI",
	"FR", "HR", "HU", "IE", "IT", "LT", "LU", "LV", "MT", "NL", "PL",
	"PT", "RO", "SE", "SI", "SK", "XI",
}

// Check if the country code is a European country code
func isEuropeanCountryCode(code string) bool {
	for _, c := range europeanCountryCodes {
		if c == code {
			return true
		}
	}
	return false
}
