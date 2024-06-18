package gobltin

import (
	"reflect"
	"testing"
)

func TestGetTinLookup(t *testing.T) {
	tests := []struct {
		name         string
		countryCode  string
		expectedType reflect.Type
	}{
		{
			name:         "European Country Code",
			countryCode:  "ES",
			expectedType: reflect.TypeOf((*VIESLookup)(nil)).Elem(),
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
			validator := GetTinLookup(tt.countryCode)

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
