package tin

import (
	"context"
	"testing"

	"github.com/invopop/gobl.tin/test"
	"github.com/invopop/gobl/bill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupTin(t *testing.T) {
	tests := []struct {
		name          string
		file          string
		expectedValid bool
		expectedError error
	}{
		{
			name:          "Valid invoice",
			file:          "test/data/invoice-valid.json",
			expectedValid: false,
			expectedError: ErrInvalid.WithMessage("Supplier: TIN is invalid"),
		},
		{
			name:          "No customer",
			file:          "test/data/invoice-no-customer.json",
			expectedValid: false,
			expectedError: ErrInput.WithMessage("no customer found"),
		},
		{
			name:          "No tax ID",
			file:          "test/data/invoice-no-taxid.json",
			expectedValid: false,
			expectedError: ErrInput.WithMessage("Customer: no tax ID provided"),
		},
		{
			name:          "Invalid Country",
			file:          "test/data/invoice-invalid-country.json",
			expectedValid: false,
			expectedError: ErrNotSupported.WithMessage("Supplier: country code not supported"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := test.LoadTestEnvelope(tt.file)
			require.NoError(t, err)
			inv, ok := env.Extract().(*bill.Invoice)
			require.True(t, ok)

			ctx := context.Background()
			c := New()

			result, err := c.Lookup(ctx, inv)

			assert.Equal(t, tt.expectedValid, result)
			if err == nil {
				assert.Nil(t, tt.expectedError)
			} else {
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				assert.IsType(t, tt.expectedError, err)
			}

			/*resultCust, errCust := c.Lookup(ctx, customer)
			resultSupp, errSupp := c.Lookup(ctx, supplier)

			assert.Equal(t, tt.expectedValid[0], resultCust)
			assert.Equal(t, tt.expectedValid[1], resultSupp)
			if errCust == nil {
				assert.Nil(t, tt.expectedError[0])
			} else {
				assert.Equal(t, tt.expectedError[0].Error(), errCust.Error())
			}
			if errSupp == nil {
				assert.Nil(t, tt.expectedError[1])
			} else {
				assert.Equal(t, tt.expectedError[1].Error(), errSupp.Error())
			}*/
		})
	}

}
