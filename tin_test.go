package tin

import (
	"context"
	"testing"

	"github.com/invopop/gobl.tin/api"
	"github.com/invopop/gobl.tin/test"
	"github.com/invopop/gobl/bill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupTin(t *testing.T) {
	tests := []struct {
		name          string
		file          string
		expectedValid []bool
		expectedError []error
	}{
		{
			name:          "Valid invoice",
			file:          "test/data/invoice-valid.json",
			expectedValid: []bool{true, false},
			expectedError: []error{nil, nil},
		},
		{
			name:          "No customer",
			file:          "test/data/invoice-no-customer.json",
			expectedValid: []bool{false, false},
			expectedError: []error{api.ErrInput.WithMessage("no party provided"), nil},
		},
		{
			name:          "No tax ID",
			file:          "test/data/invoice-no-taxid.json",
			expectedValid: []bool{false, false},
			expectedError: []error{api.ErrInput.WithMessage("no tax ID provided"), nil},
		},
		{
			name:          "Invalid Country",
			file:          "test/data/invoice-invalid-country.json",
			expectedValid: []bool{true, false},
			expectedError: []error{nil, api.ErrNotSupported.WithMessage("country code not supported")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := test.LoadTestEnvelope(tt.file)
			require.NoError(t, err)
			inv, ok := env.Extract().(*bill.Invoice)
			require.True(t, ok)

			customer := inv.Customer
			supplier := inv.Supplier
			ctx := context.Background()
			c := New()

			resultCust, errCust := c.Lookup(ctx, customer)
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
			}
		})
	}

}
