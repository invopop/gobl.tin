// Package gobltin implements the lookup of TIN numbers
package gobltin

import (
	"context"

	"github.com/invopop/gobl/org"
)

// LookupTin checks the validity of the TIN number for the customer and/or the supplier in an invoice.
// The function returns a list of PartyTinResponse, one for each party type requested.
//
// The user can choose to validate the customer, the supplier, or both, included as an argument.
// If the party type is not provided, the function will validate only the customer.
func LookupTin(ctx context.Context, party *org.Party) (bool, error) {

	if party == nil {
		return false, ErrNoParty
	}

	// Create a new client
	c := New()
	//ctx := context.Background()

	// Validate there is a taxID
	tid := party.TaxID
	if tid == nil {
		return false, ErrTaxID.WithMessage("no tax ID provided")
	}

	response, err := c.ValidateTin(ctx, tid)
	if err != nil {
		return false, err
	}
	return response.Valid, nil

}
