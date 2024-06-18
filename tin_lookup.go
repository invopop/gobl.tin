package main

// Interface for all lookup functions
type CheckTinResponse struct {
	Valid       bool   `json:"valid"`
	CountryCode string `json:"countryCode"`
	TinNumber   string `json:"vatNumber"`
	RequestDate string `json:"requestDate"`
}

type TinLookup interface {
	LookupTin(countryCode, tinNumber string) (*CheckTinResponse, error)
}
