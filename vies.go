package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
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
		log.Error().Err(err).Msg("Error marshalling JSON")
		return nil, err
	}

	resp, err := http.Post(VIES_API_URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Error().Err(err).Msg("Error sending POST request")
		return nil, err
	}
	defer resp.Body.Close()

	var vatResponse CheckTinResponse
	if resp.StatusCode == http.StatusOK {
		err = json.NewDecoder(resp.Body).Decode(&vatResponse)
		if err != nil {
			log.Error().Err(err).Msg("Error decoding JSON")
			return nil, err
		}
		return &vatResponse, nil
	} else if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusInternalServerError {
		var commonResp CommonResponse
		err = json.NewDecoder(resp.Body).Decode(&commonResp)
		if err != nil {
			log.Error().Err(err).Msg("Error decoding JSON")
			return nil, fmt.Errorf("received %d status code with unknown body", resp.StatusCode)
		}
		log.Error().Msgf("Received %d status code: %s", resp.StatusCode, commonResp.Message)
		return nil, fmt.Errorf("received %d status code: %s", resp.StatusCode, commonResp.Message)
	} else {
		log.Error().Msgf("Received unexpected %d status code", resp.StatusCode)
		return nil, fmt.Errorf("received unexpected %d status code", resp.StatusCode)
	}
}
