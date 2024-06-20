// Package gobltin implements the lookup of TIN numbers
package gobltin

import (
	"context"
	"errors"
	"fmt"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl/bill"
)

// PartyType defines the type of party to validate.
type PartyType int

const (
	// Customer is a constant used to validate only the customer party in the invoice
	Customer PartyType = iota
	// Supplier is a constant used to validate only the supplier party in the invoice
	Supplier
	// Both is a constant used to validate both the customer and the supplier parties in the invoice
	Both
)

// PartyTinResponse is the response sent to the user requesting the TIN lookup.
type PartyTinResponse struct {
	Party   PartyType
	Valid   bool
	Message string
}

// LookupTin checks the validity of the TIN number for the customer and/or the supplier in an invoice.
// The function returns a list of PartyTinResponse, one for each party type requested.
//
// The user can choose to validate the customer, the supplier, or both, included as an argument.
// If the party type is not provided, the function will validate only the customer.
func LookupTin(env *gobl.Envelope, args ...PartyType) ([]*PartyTinResponse, error) {
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return nil, fmt.Errorf("invalid type %T", env.Document)
	}

	var results []*PartyTinResponse

	partyType := Customer

	// Check if the entity type is provided
	if len(args) > 0 {
		partyType = args[0]
		if partyType != Customer && partyType != Supplier && partyType != Both {
			return nil, errors.New("invalid entity type")
		}
	}

	// Create a new client and context
	c := New()
	ctx := context.Background()

	if partyType == Customer || partyType == Both {
		if inv.Customer == nil {
			results = append(results, &PartyTinResponse{
				Party:   Customer,
				Valid:   false,
				Message: "no customer found",
			})
		} else {
			tid := inv.Customer.TaxID
			if response, err := c.ValidateTin(ctx, tid); err != nil {
				if _, ok := err.(*InvalidTaxIDError); ok {
					// Tax ID error, show in response
					results = append(results, &PartyTinResponse{
						Party:   Customer,
						Valid:   false,
						Message: fmt.Sprintf("customer: %s", err.Error()),
					})
				} else {
					// Network error
					return nil, fmt.Errorf("customer: %s", err.Error())
				}
			} else {
				results = append(results, &PartyTinResponse{
					Party:   Customer,
					Valid:   response.Valid,
					Message: "customer: valid",
				})
			}
		}
		if partyType == Customer {
			return results, nil
		}
	}

	if partyType == Supplier || partyType == Both {
		if inv.Supplier == nil {
			results = append(results, &PartyTinResponse{
				Party:   Supplier,
				Valid:   false,
				Message: "no supplier found",
			})
		} else {
			tid := inv.Supplier.TaxID
			if response, err := c.ValidateTin(ctx, tid); err != nil {
				if _, ok := err.(*InvalidTaxIDError); ok {
					// Tax ID error, show in response
					results = append(results, &PartyTinResponse{
						Party:   Supplier,
						Valid:   false,
						Message: fmt.Sprintf("supplier: %s", err.Error()),
					})
				} else {
					// Network error
					return nil, fmt.Errorf("supplier: %s", err.Error())
				}
			} else {
				results = append(results, &PartyTinResponse{
					Party:   Supplier,
					Valid:   response.Valid,
					Message: "supplier: valid",
				})
			}
		}
	}

	return results, nil

}
