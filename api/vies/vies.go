// Package vies implements the API call to the VIES service to validate a TIN number
package vies

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-resty/resty/v2"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/tax"
)

// DefaultBaseURL is the production VIES REST endpoint.
const DefaultBaseURL = "https://ec.europa.eu/taxation_customs/vies/rest-api"

const checkVatPath = "/check-vat-number"

// API implements the VIES lookup.
type API struct {
	baseURL string
	conn    *resty.Client
}

// Option configures the API.
type Option func(*API)

// WithBaseURL points the API at a different endpoint. It exists so that tests
// can run against a local server instead of the live VIES service.
func WithBaseURL(url string) Option {
	return func(a *API) {
		a.baseURL = url
	}
}

// New creates a new VIES API client.
func New(opts ...Option) *API {
	a := &API{
		baseURL: DefaultBaseURL,
	}
	for _, opt := range opts {
		opt(a)
	}
	a.conn = resty.New().SetBaseURL(a.baseURL)
	return a
}

// CheckVatRequest is the request body for the VIES API
type CheckVatRequest struct {
	CountryCode l10n.TaxCountryCode `json:"countryCode"`
	VatNumber   cbc.Code            `json:"vatNumber"`
}

// CommonResponse is the response body for the VIES API
type CommonResponse struct {
	Message string `json:"message"`
}

// CheckTINResponse is the response from a TIN lookup
type CheckTINResponse struct {
	Valid       bool   `json:"valid"`
	CountryCode string `json:"countryCode"`
	TinNumber   string `json:"vatNumber"`
}

// LookupTIN validates existence of VAT number in VIES database
func (a *API) LookupTIN(ctx context.Context, tid *tax.Identity) (bool, error) {
	reqBody := CheckVatRequest{
		CountryCode: tid.Country,
		VatNumber:   tid.Code,
	}

	resp, err := a.conn.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(reqBody).
		Post(checkVatPath)

	if err != nil {
		return false, err
	}

	if !resp.IsSuccess() {
		var commonResp CommonResponse
		if err = json.Unmarshal(resp.Body(), &commonResp); err != nil {
			return false, fmt.Errorf("received %d status code with unknown body", resp.StatusCode())
		}

		code := resp.StatusCode()
		switch code {
		case 400, 500:
			return false, fmt.Errorf("received %d status code: %s", code, commonResp.Message)
		}

		return false, fmt.Errorf("received unexpected %d status code", code)
	}

	var vatResponse CheckTINResponse
	if err = json.Unmarshal(resp.Body(), &vatResponse); err != nil {
		return false, err
	}

	return vatResponse.Valid, nil
}
