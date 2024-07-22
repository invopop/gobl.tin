// Package gobltin implements the lookup of TIN numbers
package tin

import (
	"context"

	"github.com/invopop/gobl.tin/api"
	"github.com/invopop/gobl/org"
)

// Client encapsulates the TIN lookup logic.
type Client struct{}

// New creates a new Client instance.
func New() *Client {
	return &Client{}
}

// Add a cache to the client?

// Lookup checks the validity of the TIN number for the customer and/or the supplier in an invoice.
// The function returns a list of PartyTinResponse, one for each party type requested.
//
// The user can choose to validate the customer, the supplier, or both, included as an argument.
// If the party type is not provided, the function will validate only the customer.
func (c *Client) Lookup(ctx context.Context, party *org.Party) (bool, error) {

	if party == nil {
		return false, api.ErrInput.WithMessage("no party provided")
	}

	// Validate there is a taxID
	tid := party.TaxID
	if tid == nil {
		return false, api.ErrInput.WithMessage("no tax ID provided")
	}

	validator := GetLookupAPI(tid.Country)
	if validator == nil {
		return false, api.ErrNotSupported.WithMessage("country code not supported")
	}

	response, err := validator.LookupTIN(ctx, tid)
	if err != nil {
		return false, err
	}
	return response, nil

}
