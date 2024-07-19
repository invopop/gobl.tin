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
	validator := GetTinLookup(tid.Country)
	if validator == nil {
		return nil, ErrNotSupported.WithMessage("country code not supported")
	}

	response, err := validator.LookupTin(ctx, tid)
	if err != nil {
		return nil, err
	}

	return response, nil
}
