// Package tin contains the first function that a user will call to lookup a TIN number.
package tin

import (
	"context"

	"github.com/invopop/gobl.tin/api"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	cmap "github.com/orcaman/concurrent-map/v2"
)

// Client encapsulates the TIN lookup logic.
type Client struct {
	cache cmap.ConcurrentMap[string, bool]
}

// New creates a new Client instance.
func New() *Client {
	return &Client{
		cache: cmap.New[bool](),
	}
}

// Lookup checks the validity of the TIN number
//
// There are three cases:
//
// 1. If the input is an invoice, it will check the TIN of the customer and supplier
//
// 2. If the input is a party, it will check the TIN of the party.
//
// 3. If the input is a tax ID, it will check the TIN.
func (c *Client) Lookup(ctx context.Context, in any) error {

	// 3 cases: invoice, party or tax ID
	switch in := in.(type) {
	case *bill.Invoice:
		return c.lookupInvoice(ctx, in)
	case *org.Party:
		return c.lookupParty(ctx, in)
	case *tax.Identity:
		return c.lookupTaxID(ctx, in)
	default:
		return ErrInput.WithMessage("invalid input type")
	}
}

func (c *Client) lookupTaxID(ctx context.Context, tid *tax.Identity) error {
	// check the cache
	var response bool
	var err error
	var ok bool

	key := tid.String()

	if response, ok = c.cache.Get(key); !ok {
		validator := api.GetLookupAPI(tid.Country)
		if validator == nil {
			return ErrNotSupported.WithMessage("country code not supported")
		}

		response, err = validator.LookupTIN(ctx, tid)
		if err != nil {
			return ErrNetwork.WithMessage(err.Error())
		}

		c.cache.Set(key, response)
	}

	if !response {
		return ErrInvalid.WithMessage("TIN is invalid")
	}
	return nil
}

func (c *Client) lookupParty(ctx context.Context, party *org.Party) error {
	tid := party.TaxID
	if tid == nil {
		return ErrInput.WithMessage("no tax ID provided")
	}

	return c.lookupTaxID(ctx, tid)
}

func (c *Client) lookupInvoice(ctx context.Context, inv *bill.Invoice) error {
	customer := inv.Customer
	if customer == nil {
		return ErrInput.WithMessage("no customer found")
	}
	errCust := c.lookupParty(ctx, customer)
	if errCust != nil {
		if e, ok := errCust.(*Error); ok {
			return e.WithMessage("Customer: " + e.Error())
		}
		return errCust
	}

	supplier := inv.Supplier
	if supplier == nil {
		return ErrInput.WithMessage("no supplier found")
	}
	errSupp := c.lookupParty(ctx, supplier)
	if errSupp != nil {
		if e, ok := errSupp.(*Error); ok {
			return e.WithMessage("Supplier: " + e.Error())
		}
		return errSupp
	}

	return nil
}
