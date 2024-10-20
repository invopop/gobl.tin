package vies

import (
	"context"
	"testing"

	"github.com/invopop/gobl/tax"
)

func TestViesLookup(t *testing.T) {
	tests := []struct {
		name          string
		tin           *tax.Identity
		expectError   bool
		expectedValid bool
	}{
		{
			name:          "Valid VAT number",
			tin:           &tax.Identity{Country: "DE", Code: "282741168"},
			expectError:   false,
			expectedValid: true,
		},
		{
			name:          "Invalid VAT number",
			tin:           &tax.Identity{Country: "CZ", Code: "INVALID"},
			expectError:   false,
			expectedValid: false,
		},
		{
			name:        "Empty VAT number",
			tin:         &tax.Identity{Country: "IT", Code: ""},
			expectError: true,
		},
		{
			name:        "Symbols in VAT number",
			tin:         &tax.Identity{Country: "DE", Code: "H45%ˆ#"},
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := API{}
			ctx := context.Background()
			resp, err := validator.LookupTIN(ctx, tt.tin)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if resp != tt.expectedValid {
					t.Errorf("Expected valid %v, got %v", tt.expectedValid, resp)
				}
			}
		})
	}
}
