package gobltin

import (
	"errors"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl/bill"
)

type TinNumber struct {
	CountryCode string
	TinNumber   string
}

// NewTinNumber creates a new TinNumber object from an envelope
func NewTinNumber(env *gobl.Envelope) (*TinNumber, error) {
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

	tin := &TinNumber{
		CountryCode: inv.Customer.TaxID.Country.String(),
		TinNumber:   inv.Customer.TaxID.Code.String(),
	}

	return tin, nil

}

func (t *TinNumber) Lookup() (*CheckTinResponse, error) {
	// Get the validator for the country code
	validator := GetTinLookup(t.CountryCode)
	if validator == nil {
		return nil, errors.New("no validator found for country code")
	}

	response, err := validator.LookupTin(t.CountryCode, t.TinNumber)
	if err != nil {
		return nil, err
	}

	return response, nil
}
