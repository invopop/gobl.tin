package gobltin

import (
	"context"
	"reflect"
	"testing"

	"github.com/invopop/gobl/tax"
)

func TestValidateTin(t *testing.T) {
	tests := []struct {
		name             string
		tid              *tax.Identity
		expectedResponse *CheckTinResponse
		expectedError    error
	}{
		{
			name:             "Valid VAT number",
			tid:              &tax.Identity{Country: "DE", Code: "282741168"},
			expectedResponse: &CheckTinResponse{Valid: true, CountryCode: "DE", TinNumber: "282741168"},
			expectedError:    nil,
		},
		{
			name:             "Invalid VAT number",
			tid:              &tax.Identity{Country: "CZ", Code: "INVALID"},
			expectedResponse: &CheckTinResponse{Valid: false, CountryCode: "CZ", TinNumber: "INVALID"},
			expectedError:    &InvalidTaxIDError{Msg: "tax ID not found in database"},
		},
		{
			name:             "Empty VAT number",
			tid:              nil,
			expectedResponse: nil,
			expectedError:    &InvalidTaxIDError{Msg: "no tax ID provided"},
		},
		{
			name:             "Non supported country code",
			tid:              &tax.Identity{Country: "US", Code: "12345678"},
			expectedResponse: nil,
			expectedError:    &InvalidTaxIDError{Msg: "no validator found for country code"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := New()
			ctx := context.Background()
			resp, err := client.ValidateTin(ctx, tt.tid)

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else {
					if err.Error() != tt.expectedError.Error() {
						t.Errorf("Expected error %v, got %v", tt.expectedError, err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if !reflect.DeepEqual(resp, tt.expectedResponse) {
					t.Errorf("Expected response %v, got %v", tt.expectedResponse, resp)
				}
			}
		})
	}
}
