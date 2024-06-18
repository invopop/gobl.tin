package gobltin

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/joho/godotenv"
)

// The valid VAT number for Spain is stored in the .env file
func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}
}

func TestViesLookup(t *testing.T) {
	tests := []struct {
		name          string
		countryCode   string
		tinNumber     string
		expectError   bool
		expectedValid bool
	}{
		{
			name:          "Valid VAT number",
			countryCode:   "ES",
			tinNumber:     os.Getenv("VAT_TEST_NUMBER_ES"),
			expectError:   false,
			expectedValid: true,
		},
		{
			name:          "Invalid VAT number",
			countryCode:   "CZ",
			tinNumber:     "INVALID",
			expectError:   false,
			expectedValid: false,
		},
		{
			name:        "Empty VAT number",
			countryCode: "IT",
			tinNumber:   "",
			expectError: true,
		},
		{
			name:        "Symbols in VAT number",
			countryCode: "DE",
			tinNumber:   "H45%ˆ#",
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := VIESLookup{}
			resp, err := validator.LookupTin(tt.countryCode, tt.tinNumber)

			fmt.Println(resp)
			if tt.expectError {
				fmt.Println(resp)
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if resp.Valid != tt.expectedValid {
					t.Errorf("Expected valid %v, got %v", tt.expectedValid, resp.Valid)
				}
			}
		})
	}
}
