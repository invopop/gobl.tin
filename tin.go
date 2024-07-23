// Package tin contains the first function that a user will call to lookup a TIN number.
package tin

import (
	"context"
	"errors"

	"github.com/invopop/gobl.tin/api"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// Client encapsulates the TIN lookup logic.
type Client struct{}

// New creates a new Client instance.
func New() *Client {
	return &Client{}
}

// Add a cache to the client?

// Lookup checks the validity of the TIN number
//
// There are three cases:
//
// 1. If the input is an invoice, it will check the TIN of the customer and supplier
//
// 2. If the input is a party, it will check the TIN of the party.
//
// 3. If the input is a tax ID, it will check the TIN.
func (c *Client) Lookup(ctx context.Context, in any) (bool, error) {

	// 3 cases: invoice, party or tax ID
	switch in := in.(type) {
	case *bill.Invoice:
		return c.lookupInvoice(ctx, in)
	case *org.Party:
		return c.lookupParty(ctx, in)
	case *tax.Identity:
		return c.lookupTaxID(ctx, in)
	default:
		return false, ErrInput.WithMessage("invalid input type")
	}
}

func (c *Client) lookupTaxID(ctx context.Context, tid *tax.Identity) (bool, error) {
	validator := api.GetLookupAPI(tid.Country)
	if validator == nil {
		return false, ErrNotSupported.WithMessage("country code not supported")
	}

	response, err := validator.LookupTIN(ctx, tid)
	if err != nil {
		return false, ErrNetwork.WithMessage(err.Error())
	}
	if !response {
		return false, ErrInvalid.WithMessage("TIN is invalid")
	}
	return response, nil
}

func (c *Client) lookupParty(ctx context.Context, party *org.Party) (bool, error) {
	tid := party.TaxID
	if tid == nil {
		return false, ErrInput.WithMessage("no tax ID provided")
	}

	return c.lookupTaxID(ctx, tid)
}

func (c *Client) lookupInvoice(ctx context.Context, inv *bill.Invoice) (bool, error) {
	customer := inv.Customer
	if customer == nil {
		return false, ErrInput.WithMessage("no customer found")
	}
	_, errCust := c.lookupParty(ctx, customer)
	if errCust != nil {
		var e *Error
		if errors.As(errCust, &e) {
			return false, e.WithMessage("Customer: " + e.Error())
		}
		return false, errCust
	}

	supplier := inv.Supplier
	if supplier == nil {
		return false, ErrInput.WithMessage("no supplier found")
	}
	_, errSupp := c.lookupParty(ctx, supplier)
	if errSupp != nil {
		var e *Error
		if errors.As(errSupp, &e) {
			return false, e.WithMessage("Supplier: " + e.Error())
		}
		return false, errSupp
	}

	return true, nil
}
