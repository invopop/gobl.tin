// Package tin looks up tax identities in GOBL documents against official
// registries, such as VIES for EU VAT numbers.
package tin

import (
	"context"
	"fmt"

	"github.com/invopop/gobl.tin/api"
	"github.com/invopop/gobl.tin/api/vies"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// Result is the outcome of a registry lookup. An invalid TIN is a Result with
// Valid false and a nil error, never an error.
type Result = api.Result

// ApplyOptions controls how much of a Result is written back into a party by
// Result.ApplyTo.
type ApplyOptions = api.ApplyOptions

// Changes reports what Result.ApplyTo altered on a party.
type Changes = api.Changes

// NameMatches reports whether two names agree once case, punctuation and
// surrounding whitespace are folded.
func NameMatches(a, b string) bool {
	return api.NameMatches(a, b)
}

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
// The Client holds no cache, so every call reaches the registry. Caching, and
// how long an answer stays fresh, is the consuming application's decision.
// The registry clients themselves are built once per Client and reused, so
// lookups share connections.
type Client struct {
	viesOpts []vies.Option
	vies     api.LookupAPI
}

// Option configures the Client.
type Option func(*Client)

// WithVIESOptions passes options through to the VIES client the Client
// builds, for example a timeout or a different base URL.
func WithVIESOptions(opts ...vies.Option) Option {
	return func(c *Client) {
		c.viesOpts = append(c.viesOpts, opts...)
	}
}

// WithVIES replaces the VIES registry client entirely. It exists so that
// tests and consumers can inject their own implementation.
func WithVIES(registry api.LookupAPI) Option {
	return func(c *Client) {
		c.vies = registry
	}
}

// New creates a new Client instance.
func New(opts ...Option) *Client {
	c := new(Client)
	for _, opt := range opts {
		opt(c)
	}
	if c.vies == nil {
		c.vies = vies.New(c.viesOpts...)
	}
	return c
}

// LookupIdentity checks the tax identity against the registry for its country.
func (c *Client) LookupIdentity(ctx context.Context, tid *tax.Identity) (*Result, error) {
	if tid == nil {
		return nil, ErrInput.WithMessage("no tax ID provided")
	}
	if tid.Code == "" {
		return nil, ErrInput.WithMessage("no tax ID code provided")
	}
	registry := c.lookupAPIFor(tid.Country)
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
