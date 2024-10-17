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

const viesAPIURL = "https://ec.europa.eu/taxation_customs/vies/rest-api//check-vat-number"

// API is a struct that implements the VIES lookup and inherits from LookupAPI
type API struct{}

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
func (v API) LookupTIN(c context.Context, tid *tax.Identity) (bool, error) {
	reqBody := CheckVatRequest{
		CountryCode: tid.Country,
		VatNumber:   tid.Code,
	}

	client := resty.New()
	resp, err := client.R().
		SetContext(c).
		SetHeader("Content-Type", "application/json").
		SetBody(reqBody).
		Post(viesAPIURL)

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
