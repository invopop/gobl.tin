package tin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/invopop/gobl.tin/api"
	"github.com/invopop/gobl.tin/api/vies"
	"github.com/invopop/gobl.tin/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/l10n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockedClient returns a Client whose EU lookups go to a test server that
// always reports the number as valid.
func mockedClient(t *testing.T) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"countryCode":"DE","vatNumber":"282741168","valid":true,"name":"---","address":"---"}`))
	}))
	t.Cleanup(srv.Close)

	c := New()
	c.apiFor = func(cc l10n.TaxCountryCode) api.LookupAPI {
		if api.GetLookupAPI(cc) == nil {
			return nil
		}
		return vies.New(vies.WithBaseURL(srv.URL))
	}
	return c
}

func TestLookupTin(t *testing.T) {
	tests := []struct {
		name          string
		file          string
		expectedError error
	}{
		{
			name:          "Valid invoice",
			file:          "test/data/invoice-valid.json",
			expectedError: nil,
		},
		{
			name:          "No customer",
			file:          "test/data/invoice-no-customer.json",
			expectedError: ErrInput.WithMessage("no customer found"),
		},
		{
			name:          "No tax ID",
			file:          "test/data/invoice-no-taxid.json",
			expectedError: ErrInput.WithMessage("Customer: no tax ID provided"),
		},
		{
			name:          "Invalid Country",
			file:          "test/data/invoice-invalid-country.json",
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
			c := mockedClient(t)

			err = c.Lookup(ctx, inv)

			if tt.expectedError == nil {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
				assert.IsType(t, tt.expectedError, err)
			}
		})
	}
}

func TestLookupTinInvalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"countryCode":"DE","vatNumber":"111111125","valid":false,"name":"---","address":"---"}`))
	}))
	t.Cleanup(srv.Close)

	c := New()
	c.apiFor = func(_ l10n.TaxCountryCode) api.LookupAPI {
		return vies.New(vies.WithBaseURL(srv.URL))
	}

	env, err := test.LoadTestEnvelope("test/data/invoice-valid.json")
	require.NoError(t, err)
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)

	err = c.Lookup(context.Background(), inv)
	require.Error(t, err)
	assert.Equal(t, ErrInvalid.WithMessage("Customer: TIN is invalid").Error(), err.Error())
}
