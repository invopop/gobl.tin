package vies

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// viesValidBody is the real VIES response shape for a valid Spanish VAT
// number. Spain masks the trader name and address with "---".
const viesValidBody = `{
	"countryCode": "ES",
	"vatNumber": "B85905495",
	"requestDate": "2026-09-22T10:00:00.000Z",
	"valid": true,
	"requestIdentifier": "",
	"name": "---",
	"address": "---",
	"traderName": "---",
	"traderStreet": null,
	"traderPostalCode": null,
	"traderCity": null,
	"traderCompanyType": null,
	"traderNameMatch": "NOT_PROCESSED",
	"traderStreetMatch": "NOT_PROCESSED",
	"traderPostalCodeMatch": "NOT_PROCESSED",
	"traderCityMatch": "NOT_PROCESSED",
	"traderCompanyTypeMatch": "NOT_PROCESSED"
}`

const viesInvalidBody = `{
	"countryCode": "CZ",
	"vatNumber": "00000000",
	"requestDate": "2026-09-22T10:00:00.000Z",
	"valid": false,
	"requestIdentifier": "",
	"name": "---",
	"address": "---",
	"traderNameMatch": "NOT_PROCESSED"
}`

// serve starts a test server that always answers with the given status and
// body, and returns an API pointed at it.
func serve(t *testing.T, status int, body string) *API {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(WithBaseURL(srv.URL))
}

func TestLookupTIN(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantValid  bool
		wantErr    string
		wantAnyErr bool
	}{
		{
			name:      "valid response",
			status:    http.StatusOK,
			body:      viesValidBody,
			wantValid: true,
		},
		{
			name:      "invalid response",
			status:    http.StatusOK,
			body:      viesInvalidBody,
			wantValid: false,
		},
		{
			name:    "bad request with message",
			status:  http.StatusBadRequest,
			body:    `{"message": "Invalid VAT number format"}`,
			wantErr: "received 400 status code: Invalid VAT number format",
		},
		{
			name:    "server error with message",
			status:  http.StatusInternalServerError,
			body:    `{"message": "MS_UNAVAILABLE"}`,
			wantErr: "received 500 status code: MS_UNAVAILABLE",
		},
		{
			name:       "malformed JSON",
			status:     http.StatusOK,
			body:       `{"valid": tru`,
			wantAnyErr: true,
		},
		{
			name:    "malformed JSON on failure status",
			status:  http.StatusBadRequest,
			body:    `<html>bad gateway</html>`,
			wantErr: "received 400 status code with unknown body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := serve(t, tt.status, tt.body)
			tid := &tax.Identity{Country: "ES", Code: "B85905495"}

			valid, err := api.LookupTIN(context.Background(), tid)

			switch {
			case tt.wantErr != "":
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
			case tt.wantAnyErr:
				require.Error(t, err)
			default:
				require.NoError(t, err)
				assert.Equal(t, tt.wantValid, valid)
			}
		})
	}
}

func TestLookupTINNetworkFailure(t *testing.T) {
	// A server that is already closed refuses the connection, which is the
	// closest offline stand-in for a network failure.
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close()
	api := New(WithBaseURL(srv.URL))

	tid := &tax.Identity{Country: "ES", Code: "B85905495"}
	_, err := api.LookupTIN(context.Background(), tid)
	require.Error(t, err)
}
