package gobltin

import (
	"context"

	"github.com/invopop/gobl/tax"
)

// CheckTinResponse is the response from a TIN lookup
type CheckTinResponse struct {
	Valid       bool   `json:"valid"`
	CountryCode string `json:"countryCode"` // Cambiar estos campos por alguno relacionado con GOBL
	TinNumber   string `json:"vatNumber"`
	RequestDate string `json:"requestDate"`
}

// TinLookup is the interface for looking up TIN numbers
type TinLookup interface {
	LookupTin(ctx context.Context, tid *tax.Identity) (*CheckTinResponse, error)
}
