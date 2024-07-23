// Package api handles all the API calls for the TIN lookup
package api

import (
	"context"

	"github.com/invopop/gobl/tax"
)

// LookupAPI is the interface for looking up TIN numbers
type LookupAPI interface {
	LookupTIN(ctx context.Context, tid *tax.Identity) (bool, error)
}
