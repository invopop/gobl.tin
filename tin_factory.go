package main

// Factory for getting all validation functions
func GetTinLookup(countryCode string) TinLookup {
	switch countryCode {
	case "ES", "DE", "FR": // List all EU country codes here
		return VIESLookup{}
	// Add cases for other countries and their specific validators
	default:
		return nil // or a default validator if applicable
	}
}
