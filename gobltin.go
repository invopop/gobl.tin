// Package gobltin implements the lookup of TIN numbers
package gobltin

import (
	"context"
	"errors"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/tax"
)

// TinNumber identifies a TIN number by country code and number
/*type TinNumber struct {
	CountryCode string
	TinNumber   string
}*/

// NewTinNumber creates a new TinNumber object from an envelope
func NewTinNumber(env *gobl.Envelope) (*tax.Identity, error) {
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return nil, errors.New("invalid document type")
	}

	// Check if the customer is set
	if inv.Customer == nil {
		return nil, errors.New("no customer found")
	}

	// Check if the customer has a tax ID
	if inv.Customer.TaxID == nil {
		return nil, errors.New("no tax ID found")
	}

	// Check if the taxId contains a country code
	if inv.Customer.TaxID.Country.String() == "" {
		return nil, errors.New("no country code found")
	}

	// Check if the taxId contains the number
	if inv.Customer.TaxID.Code.String() == "" {
		return nil, errors.New("no tax ID code found")
	}

	/*tin := &TinNumber{
		CountryCode: inv.Customer.TaxID.Country.String(),
		TinNumber:   inv.Customer.TaxID.Code.String(),
	}*/

	return inv.Customer.TaxID, nil

}

func Lookup(ctx context.Context, tid *tax.Identity) (*CheckTinResponse, error) {
	// Get the validator for the country code
	validator := GetTinLookup(tid.Country.String())
	if validator == nil {
		return nil, errors.New("no validator found for country code")
	}

	response, err := validator.LookupTin(ctx, tid)
	if err != nil {
		return nil, err
	}

	return response, nil
}
