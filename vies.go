package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

const VIES_API_URL = "https://ec.europa.eu/taxation_customs/vies/rest-api//check-vat-number"

type VIESLookup struct{}

type CheckVatRequest struct {
	CountryCode string `json:"countryCode"`
	VatNumber   string `json:"vatNumber"`
}

type CommonResponse struct {
	Message string `json:"message"`
}

// Validating existence of VAT number in VIES database
func (v VIESLookup) LookupTin(countryCode, vatNumber string) (*CheckTinResponse, error) {
	reqBody := CheckVatRequest{
		CountryCode: countryCode,
		VatNumber:   vatNumber,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, errors.New("error marshalling JSON")
	}

	resp, err := http.Post(VIES_API_URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, errors.New("error sending request")
	}
	defer resp.Body.Close()

	var vatResponse CheckTinResponse
	if resp.StatusCode == http.StatusOK {
		err = json.NewDecoder(resp.Body).Decode(&vatResponse)
		if err != nil {
			return nil, errors.New("error decoding JSON")
		}
		return &vatResponse, nil
	} else if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		var commonResp CommonResponse
		err = json.NewDecoder(resp.Body).Decode(&commonResp)
		if err != nil {
			return nil, fmt.Errorf("received %d status code with unknown body", resp.StatusCode)
		}
		return nil, fmt.Errorf("received %d status code: %s", resp.StatusCode, commonResp.Message)
	} else {
		return nil, fmt.Errorf("received unexpected %d status code", resp.StatusCode)
	}
}
