// Package tin looks up tax identities in GOBL documents against official
// registries, such as VIES for EU VAT numbers.
package tin

import (
	"context"
	"fmt"

	"github.com/invopop/gobl.tin/api"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// Result is the outcome of a registry lookup. An invalid TIN is a Result with
// Valid false and a nil error, never an error.
type Result = api.Result

// InvoiceParty selects which parties of an invoice to look up.
type InvoiceParty string

// Invoice parties that can be looked up.
const (
	InvoicePartyCustomer InvoiceParty = "customer"
	InvoicePartySupplier InvoiceParty = "supplier"
	InvoicePartyBoth     InvoiceParty = "both"
)

// InvoiceResult holds the lookup results for the requested parties of an
// invoice. A field is nil when that party was not requested.
type InvoiceResult struct {
	Customer *Result
	Supplier *Result
}

// Client dispatches TIN lookups to the registry for the identity's country.
//
// The Client is stateless: it holds no cache, so every call reaches the
// registry. Caching, and how long an answer stays fresh, is the consuming
// application's decision.
type Client struct {
	// apiFor resolves the registry for a country. It defaults to the package
	// factory and exists so that tests can point lookups at a local server.
	apiFor func(l10n.TaxCountryCode) api.LookupAPI
}

// New creates a new Client instance.
func New() *Client {
	return &Client{
		apiFor: lookupAPIFor,
	}
}

// LookupIdentity checks the tax identity against the registry for its country.
func (c *Client) LookupIdentity(ctx context.Context, tid *tax.Identity) (*Result, error) {
	if tid == nil {
		return nil, ErrInput.WithMessage("no tax ID provided")
	}
	if tid.Code == "" {
		return nil, ErrInput.WithMessage("no tax ID code provided")
	}
	registry := c.apiFor(tid.Country)
	if registry == nil {
		return nil, ErrNotSupported.WithMsgf("country code %q not supported", tid.Country)
	}
	return registry.LookupTIN(ctx, tid)
}

// LookupParty checks the tax identity of the party.
func (c *Client) LookupParty(ctx context.Context, party *org.Party) (*Result, error) {
	if party == nil {
		return nil, ErrInput.WithMessage("no party provided")
	}
	if party.TaxID == nil {
		return nil, ErrInput.WithMessage("no tax ID provided")
	}
	return c.LookupIdentity(ctx, party.TaxID)
}

// LookupInvoice checks the tax identities of the requested invoice parties.
// It stops at the first error; error messages carry the party they belong to.
func (c *Client) LookupInvoice(ctx context.Context, inv *bill.Invoice, party InvoiceParty) (*InvoiceResult, error) {
	if inv == nil {
		return nil, ErrInput.WithMessage("no invoice provided")
	}

	out := new(InvoiceResult)

	if party == InvoicePartyCustomer || party == InvoicePartyBoth {
		res, err := c.LookupParty(ctx, inv.Customer)
		if err != nil {
			return nil, fmt.Errorf("customer: %w", err)
		}
		out.Customer = res
	}

	if party == InvoicePartySupplier || party == InvoicePartyBoth {
		res, err := c.LookupParty(ctx, inv.Supplier)
		if err != nil {
			return nil, fmt.Errorf("supplier: %w", err)
		}
		out.Supplier = res
	}

	if out.Customer == nil && out.Supplier == nil {
		return nil, ErrInput.WithMsgf("invalid party %q, expected customer, supplier or both", party)
	}

	return out, nil
}
