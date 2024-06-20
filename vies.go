package gobltin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-resty/resty/v2"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/tax"
)

const viesAPIURL = "https://ec.europa.eu/taxation_customs/vies/rest-api//check-vat-number"

// VIESLookup is a struct that implements the VIES lookup and inherits from TinLookup
type VIESLookup struct{}

// CheckVatRequest is the request body for the VIES API
type CheckVatRequest struct {
	CountryCode l10n.CountryCode `json:"countryCode"`
	VatNumber   cbc.Code         `json:"vatNumber"`
}

// CommonResponse is the response body for the VIES API
type CommonResponse struct {
	Message string `json:"message"`
}

// LookupTin validates existence of VAT number in VIES database
func (v VIESLookup) LookupTin(c context.Context, tid *tax.Identity) (*CheckTinResponse, error) {
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
		return nil, errors.New("error sending request")
	}

	if resp.IsSuccess() {
		var vatResponse CheckTinResponse
		err := json.Unmarshal(resp.Body(), &vatResponse)
		if err != nil {
			return nil, errors.New("error decoding JSON")
		}
		return &vatResponse, nil
	}

	var commonResp CommonResponse
	err = json.Unmarshal(resp.Body(), &commonResp)
	if err != nil {
		return nil, fmt.Errorf("received %d status code with unknown body", resp.StatusCode())
	}

	if resp.StatusCode() == 400 || resp.StatusCode() == 500 {
		return nil, fmt.Errorf("received %d status code: %s", resp.StatusCode(), commonResp.Message)
	}

	return nil, fmt.Errorf("received unexpected %d status code", resp.StatusCode())
}
