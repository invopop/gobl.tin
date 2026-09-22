package vies

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/invopop/gobl.tin/api"
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

// viesValidNamedBody is a valid answer from a member state that discloses the
// trader details.
const viesValidNamedBody = `{
	"countryCode": "DE",
	"vatNumber": "282741168",
	"requestDate": "2026-09-22T10:00:00.000Z",
	"valid": true,
	"name": "ACME GMBH",
	"address": "MUSTERSTR. 1, 10115 BERLIN"
}`

const viesInvalidBody = `{
	"countryCode": "CZ",
	"vatNumber": "00000000",
	"requestDate": "2026-09-22T10:00:00.000Z",
	"valid": false,
	"name": "---",
	"address": "---"
}`

// serve starts a test server that always answers with the given status,
// headers and body, and returns an API pointed at it.
func serve(t *testing.T, status int, body string, header http.Header) *API {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for k, vs := range header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(WithBaseURL(srv.URL))
}

func TestLookupTIN(t *testing.T) {
	tid := &tax.Identity{Country: "ES", Code: "B85905495"}

	t.Run("valid with masked details", func(t *testing.T) {
		a := serve(t, http.StatusOK, viesValidBody, nil)
		res, err := a.LookupTIN(context.Background(), tid)
		require.NoError(t, err)
		assert.True(t, res.Valid)
		assert.Empty(t, res.Name)
		assert.Empty(t, res.Address)
		assert.Equal(t, Source, res.Source)
	})

	t.Run("valid with disclosed details", func(t *testing.T) {
		a := serve(t, http.StatusOK, viesValidNamedBody, nil)
		res, err := a.LookupTIN(context.Background(), tid)
		require.NoError(t, err)
		assert.True(t, res.Valid)
		assert.Equal(t, "ACME GMBH", res.Name)
		assert.Equal(t, "MUSTERSTR. 1, 10115 BERLIN", res.Address)
	})

	t.Run("invalid number is not an error", func(t *testing.T) {
		a := serve(t, http.StatusOK, viesInvalidBody, nil)
		res, err := a.LookupTIN(context.Background(), tid)
		require.NoError(t, err)
		assert.False(t, res.Valid)
	})

	t.Run("bad request maps to input error", func(t *testing.T) {
		a := serve(t, http.StatusBadRequest, `{"message": "Invalid VAT number format"}`, nil)
		res, err := a.LookupTIN(context.Background(), tid)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, api.ErrInput)
		assert.Contains(t, err.Error(), "Invalid VAT number format")
	})

	t.Run("server error maps to network error", func(t *testing.T) {
		a := serve(t, http.StatusInternalServerError, `{"message": "MS_UNAVAILABLE"}`, nil)
		_, err := a.LookupTIN(context.Background(), tid)
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrNetwork)
		assert.Contains(t, err.Error(), "MS_UNAVAILABLE")
	})

	t.Run("failure with unreadable body", func(t *testing.T) {
		a := serve(t, http.StatusBadGateway, `<html>bad gateway</html>`, nil)
		_, err := a.LookupTIN(context.Background(), tid)
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrNetwork)
		assert.Contains(t, err.Error(), "received 502 status code with unknown body")
	})

	t.Run("rate limited with Retry-After", func(t *testing.T) {
		a := serve(t, http.StatusTooManyRequests, `{}`, http.Header{"Retry-After": {"30"}})
		_, err := a.LookupTIN(context.Background(), tid)
		require.Error(t, err)
		var rl *api.RateLimitedError
		require.True(t, errors.As(err, &rl))
		assert.Equal(t, 30*time.Second, rl.RetryAfter)
	})

	t.Run("rate limited without Retry-After", func(t *testing.T) {
		a := serve(t, http.StatusTooManyRequests, `{}`, nil)
		_, err := a.LookupTIN(context.Background(), tid)
		require.Error(t, err)
		var rl *api.RateLimitedError
		require.True(t, errors.As(err, &rl))
		assert.Equal(t, defaultRetryAfter, rl.RetryAfter)
	})

	t.Run("malformed JSON on success status", func(t *testing.T) {
		a := serve(t, http.StatusOK, `{"valid": tru`, nil)
		_, err := a.LookupTIN(context.Background(), tid)
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrNetwork)
	})

	t.Run("network failure", func(t *testing.T) {
		// A server that is already closed refuses the connection, which is
		// the closest offline stand-in for a network failure.
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		srv.Close()
		a := New(WithBaseURL(srv.URL))
		_, err := a.LookupTIN(context.Background(), tid)
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrNetwork)
	})
}
