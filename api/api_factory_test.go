package api

import (
	"reflect"
	"testing"

	"github.com/invopop/gobl.tin/api/vies"
	"github.com/invopop/gobl/l10n"
)

func TestGetTinLookup(t *testing.T) {
	tests := []struct {
		name         string
		countryCode  l10n.TaxCountryCode
		expectedType reflect.Type
	}{
		{
			name:         "European Country Code",
			countryCode:  "ES",
			expectedType: reflect.TypeOf((*vies.API)(nil)).Elem(),
		},
		{
			name:         "Non-European Country Code",
			countryCode:  "US",
			expectedType: nil,
		},
		// Add more test cases when new validators are implemented
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := GetLookupAPI(tt.countryCode)

			if tt.expectedType == nil {
				if validator != nil {
					t.Errorf("Expected nil validator for country code %s, got %v", tt.countryCode, validator)
				}
			} else {
				actualType := reflect.TypeOf(validator)
				if actualType != tt.expectedType {
					t.Errorf("Expected validator type %s for country code %s, got %s", tt.expectedType, tt.countryCode, actualType)
				}
			}
		})
	}
}
