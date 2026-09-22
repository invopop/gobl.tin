package tin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/invopop/gobl.tin/api/vies"
	"github.com/invopop/gobl.tin/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockedClient returns a Client whose EU lookups go to a test server that
// answers with the given body.
func mockedClient(t *testing.T, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	return New(WithVIESOptions(vies.WithBaseURL(srv.URL)))
}

const validBody = `{"countryCode":"DE","vatNumber":"282741168","valid":true,"name":"ACME GMBH","address":"---"}`
const invalidBody = `{"countryCode":"DE","vatNumber":"282741168","valid":false,"name":"---","address":"---"}`

func loadInvoice(t *testing.T, file string) *bill.Invoice {
	t.Helper()
	env, err := test.LoadTestEnvelope(file)
	require.NoError(t, err)
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)
	return inv
}

func TestLookupIdentity(t *testing.T) {
	ctx := context.Background()

	t.Run("valid TIN", func(t *testing.T) {
		c := mockedClient(t, validBody)
		res, err := c.LookupIdentity(ctx, &tax.Identity{Country: "DE", Code: "282741168"})
		require.NoError(t, err)
		assert.True(t, res.Valid)
		assert.Equal(t, "ACME GMBH", res.Name)
		assert.Empty(t, res.Address)
		assert.Equal(t, vies.Source, res.Source)
	})

	t.Run("invalid TIN is not an error", func(t *testing.T) {
		c := mockedClient(t, invalidBody)
		res, err := c.LookupIdentity(ctx, &tax.Identity{Country: "DE", Code: "282741168"})
		require.NoError(t, err)
		assert.False(t, res.Valid)
	})

	t.Run("nil identity", func(t *testing.T) {
		c := mockedClient(t, validBody)
		_, err := c.LookupIdentity(ctx, nil)
		assert.ErrorIs(t, err, ErrInput)
	})

	t.Run("empty code", func(t *testing.T) {
		c := mockedClient(t, validBody)
		_, err := c.LookupIdentity(ctx, &tax.Identity{Country: "DE"})
		assert.ErrorIs(t, err, ErrInput)
	})

	t.Run("unsupported country", func(t *testing.T) {
		c := mockedClient(t, validBody)
		_, err := c.LookupIdentity(ctx, &tax.Identity{Country: "US", Code: "123456789"})
		assert.ErrorIs(t, err, ErrNotSupported)
	})
}

func TestLookupParty(t *testing.T) {
	ctx := context.Background()

	t.Run("valid party", func(t *testing.T) {
		c := mockedClient(t, validBody)
		party := &org.Party{
			Name:  "ACME GmbH",
			TaxID: &tax.Identity{Country: "DE", Code: "282741168"},
		}
		res, err := c.LookupParty(ctx, party)
		require.NoError(t, err)
		assert.True(t, res.Valid)
	})

	t.Run("nil party", func(t *testing.T) {
		c := mockedClient(t, validBody)
		_, err := c.LookupParty(ctx, nil)
		assert.ErrorIs(t, err, ErrInput)
	})

	t.Run("no tax ID", func(t *testing.T) {
		c := mockedClient(t, validBody)
		_, err := c.LookupParty(ctx, &org.Party{Name: "ACME GmbH"})
		assert.ErrorIs(t, err, ErrInput)
	})
}

func TestLookupInvoice(t *testing.T) {
	ctx := context.Background()

	t.Run("both parties valid", func(t *testing.T) {
		c := mockedClient(t, validBody)
		inv := loadInvoice(t, "test/data/invoice-valid.json")
		res, err := c.LookupInvoice(ctx, inv, InvoicePartyBoth)
		require.NoError(t, err)
		require.NotNil(t, res.Customer)
		require.NotNil(t, res.Supplier)
		assert.True(t, res.Customer.Valid)
		assert.True(t, res.Supplier.Valid)
	})

	t.Run("customer only", func(t *testing.T) {
		c := mockedClient(t, validBody)
		inv := loadInvoice(t, "test/data/invoice-valid.json")
		res, err := c.LookupInvoice(ctx, inv, InvoicePartyCustomer)
		require.NoError(t, err)
		require.NotNil(t, res.Customer)
		assert.Nil(t, res.Supplier)
	})

	t.Run("supplier only", func(t *testing.T) {
		c := mockedClient(t, validBody)
		inv := loadInvoice(t, "test/data/invoice-valid.json")
		res, err := c.LookupInvoice(ctx, inv, InvoicePartySupplier)
		require.NoError(t, err)
		assert.Nil(t, res.Customer)
		require.NotNil(t, res.Supplier)
	})

	t.Run("invalid TIN reported in result", func(t *testing.T) {
		c := mockedClient(t, invalidBody)
		inv := loadInvoice(t, "test/data/invoice-valid.json")
		res, err := c.LookupInvoice(ctx, inv, InvoicePartyBoth)
		require.NoError(t, err)
		assert.False(t, res.Customer.Valid)
		assert.False(t, res.Supplier.Valid)
	})

	t.Run("no customer", func(t *testing.T) {
		c := mockedClient(t, validBody)
		inv := loadInvoice(t, "test/data/invoice-no-customer.json")
		_, err := c.LookupInvoice(ctx, inv, InvoicePartyCustomer)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInput)
		assert.Contains(t, err.Error(), "customer")
	})

	t.Run("no customer tax ID", func(t *testing.T) {
		c := mockedClient(t, validBody)
		inv := loadInvoice(t, "test/data/invoice-no-taxid.json")
		_, err := c.LookupInvoice(ctx, inv, InvoicePartyBoth)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInput)
		assert.Contains(t, err.Error(), "customer")
	})

	t.Run("unsupported supplier country", func(t *testing.T) {
		c := mockedClient(t, validBody)
		inv := loadInvoice(t, "test/data/invoice-invalid-country.json")
		_, err := c.LookupInvoice(ctx, inv, InvoicePartyBoth)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrNotSupported)
		assert.Contains(t, err.Error(), "supplier")
	})

	t.Run("nil invoice", func(t *testing.T) {
		c := mockedClient(t, validBody)
		_, err := c.LookupInvoice(ctx, nil, InvoicePartyBoth)
		assert.ErrorIs(t, err, ErrInput)
	})

	t.Run("unknown party selector", func(t *testing.T) {
		c := mockedClient(t, validBody)
		inv := loadInvoice(t, "test/data/invoice-valid.json")
		_, err := c.LookupInvoice(ctx, inv, "everyone")
		assert.ErrorIs(t, err, ErrInput)
	})
}

// fakeRegistry is a canned api.LookupAPI implementation for injection tests.
type fakeRegistry struct {
	res *Result
}

func (f *fakeRegistry) LookupTIN(_ context.Context, _ *tax.Identity) (*Result, error) {
	return f.res, nil
}

func TestNewClient(t *testing.T) {
	t.Run("reuses one registry client across lookups", func(t *testing.T) {
		c := New()
		assert.Same(t, c.lookupAPIFor("ES"), c.lookupAPIFor("DE"))
		assert.Nil(t, c.lookupAPIFor("US"))
	})

	t.Run("WithVIES injects a registry", func(t *testing.T) {
		fake := &fakeRegistry{res: &Result{Valid: true, Source: "fake"}}
		c := New(WithVIES(fake))
		res, err := c.LookupIdentity(context.Background(), &tax.Identity{Country: "DE", Code: "282741168"})
		require.NoError(t, err)
		assert.Equal(t, "fake", string(res.Source))
	})
}
