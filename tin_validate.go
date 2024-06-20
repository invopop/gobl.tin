package gobltin

import (
	"context"

	"github.com/invopop/gobl/tax"
)

// Client encapsulates the TIN lookup logic.
type Client struct{}

// New creates a new Client instance.
func New() *Client {
	return &Client{}
}

// ValidateTin checks the existence of a TIN number.
// The function returns a CheckTinResponse with the result of the lookup.
func (c *Client) ValidateTin(ctx context.Context, tid *tax.Identity) (*CheckTinResponse, error) {
	// Get the validator for the country code
	if tid == nil {
		return nil, &InvalidTaxIDError{Msg: "no tax ID provided"}
	}
	validator := GetTinLookup(tid.Country)
	if validator == nil {
		return nil, &InvalidTaxIDError{Msg: "no validator found for country code"}
	}

	response, err := validator.LookupTin(ctx, tid)
	if err != nil {
		return nil, &NetworkError{Msg: err.Error()}
	}

	if !response.Valid {
		return response, &InvalidTaxIDError{Msg: "tax ID not found in database"}
	}

	return response, nil
}
