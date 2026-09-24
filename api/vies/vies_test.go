package vies

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/invopop/gobl.tin/api"
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

func TestVerifyTransport(t *testing.T) {
	tid := api.Identifier{Path: api.PathTaxID, Country: "ES", Code: "B85905495"}

	t.Run("missing echo falls back to the request identifier", func(t *testing.T) {
		a := serve(t, http.StatusOK, `{"valid": true, "name": "ACME GMBH"}`, nil)
		res, err := a.Verify(context.Background(), tid)
		require.NoError(t, err)
		require.NotNil(t, res.TaxID)
		assert.Equal(t, tid.Country, res.TaxID.Country)
		assert.Equal(t, tid.Code, res.TaxID.Code)
	})

	t.Run("bad request maps to input error", func(t *testing.T) {
		a := serve(t, http.StatusBadRequest, `{"message": "Invalid VAT number format"}`, nil)
		res, err := a.Verify(context.Background(), tid)
		require.Error(t, err)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, api.ErrInput)
		assert.Contains(t, err.Error(), "Invalid VAT number format")
		var e *api.Error
		require.True(t, errors.As(err, &e))
		assert.Equal(t, "400", e.Code())
	})

	t.Run("server error maps to server error with code", func(t *testing.T) {
		a := serve(t, http.StatusInternalServerError, `{"message": "MS_UNAVAILABLE"}`, nil)
		_, err := a.Verify(context.Background(), tid)
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrServer)
		assert.NotErrorIs(t, err, api.ErrNetwork)
		assert.Contains(t, err.Error(), "MS_UNAVAILABLE")
		var e *api.Error
		require.True(t, errors.As(err, &e))
		assert.Equal(t, "500", e.Code())
	})

	t.Run("failure with unreadable body", func(t *testing.T) {
		a := serve(t, http.StatusBadGateway, `<html>bad gateway</html>`, nil)
		_, err := a.Verify(context.Background(), tid)
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrServer)
		var e *api.Error
		require.True(t, errors.As(err, &e))
		assert.Equal(t, "502", e.Code())
		assert.Contains(t, err.Error(), "<html>bad gateway</html>")
	})

	t.Run("rate limited with Retry-After", func(t *testing.T) {
		a := serve(t, http.StatusTooManyRequests, `{}`, http.Header{"Retry-After": {"30"}})
		_, err := a.Verify(context.Background(), tid)
		require.Error(t, err)
		var rl *api.RateLimitedError
		require.True(t, errors.As(err, &rl))
		assert.Equal(t, 30*time.Second, rl.RetryAfter)
	})

	t.Run("rate limited with Retry-After zero", func(t *testing.T) {
		a := serve(t, http.StatusTooManyRequests, `{}`, http.Header{"Retry-After": {"0"}})
		_, err := a.Verify(context.Background(), tid)
		require.Error(t, err)
		var rl *api.RateLimitedError
		require.True(t, errors.As(err, &rl))
		assert.Equal(t, time.Duration(0), rl.RetryAfter)
	})

	t.Run("rate limited with Retry-After HTTP date", func(t *testing.T) {
		when := time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat)
		a := serve(t, http.StatusTooManyRequests, `{}`, http.Header{"Retry-After": {when}})
		_, err := a.Verify(context.Background(), tid)
		require.Error(t, err)
		var rl *api.RateLimitedError
		require.True(t, errors.As(err, &rl))
		assert.Greater(t, rl.RetryAfter, 60*time.Second)
		assert.LessOrEqual(t, rl.RetryAfter, 90*time.Second)
	})

	t.Run("rate limited with Retry-After date in the past", func(t *testing.T) {
		when := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
		a := serve(t, http.StatusTooManyRequests, `{}`, http.Header{"Retry-After": {when}})
		_, err := a.Verify(context.Background(), tid)
		require.Error(t, err)
		var rl *api.RateLimitedError
		require.True(t, errors.As(err, &rl))
		assert.Equal(t, time.Duration(0), rl.RetryAfter)
	})

	t.Run("rate limited without Retry-After", func(t *testing.T) {
		a := serve(t, http.StatusTooManyRequests, `{}`, nil)
		_, err := a.Verify(context.Background(), tid)
		require.Error(t, err)
		var rl *api.RateLimitedError
		require.True(t, errors.As(err, &rl))
		assert.Equal(t, defaultRetryAfter, rl.RetryAfter)
	})

	t.Run("missing validity field is not an answer", func(t *testing.T) {
		for _, body := range []string{`{}`, `null`, `{"name": "ACME GMBH"}`} {
			a := serve(t, http.StatusOK, body, nil)
			res, err := a.Verify(context.Background(), tid)
			require.Error(t, err, body)
			assert.Nil(t, res, body)
			assert.ErrorIs(t, err, api.ErrNetwork, body)
			var e *api.Error
			require.True(t, errors.As(err, &e), body)
			assert.Equal(t, "200", e.Code(), body)
		}
	})

	t.Run("rate limited with a huge Retry-After clamps", func(t *testing.T) {
		a := serve(t, http.StatusTooManyRequests, `{}`, http.Header{"Retry-After": {"10000000000000000"}})
		_, err := a.Verify(context.Background(), tid)
		require.Error(t, err)
		var rl *api.RateLimitedError
		require.True(t, errors.As(err, &rl))
		assert.Positive(t, rl.RetryAfter)
	})

	t.Run("malformed JSON on success status", func(t *testing.T) {
		a := serve(t, http.StatusOK, `{"valid": tru`, nil)
		_, err := a.Verify(context.Background(), tid)
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrNetwork)
	})

	t.Run("sends identifying user agent", func(t *testing.T) {
		var got string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Get("User-Agent")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(viesValidBody))
		}))
		t.Cleanup(srv.Close)
		a := New(WithBaseURL(srv.URL))
		_, err := a.Verify(context.Background(), tid)
		require.NoError(t, err)
		assert.Equal(t, userAgent, got)
	})

	t.Run("timeout maps to network error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(2 * time.Second):
			}
			_, _ = w.Write([]byte(viesValidBody))
		}))
		t.Cleanup(srv.Close)
		a := New(WithBaseURL(srv.URL), WithTimeout(50*time.Millisecond))
		_, err := a.Verify(context.Background(), tid)
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrNetwork)
	})

	t.Run("invalid base URL fails at first call", func(t *testing.T) {
		for _, bad := range []string{"not a url", "ftp://example.com", "https:foo", "https:///path", "https://:443"} {
			a := New(WithBaseURL(bad))
			_, err := a.Verify(context.Background(), tid)
			require.Error(t, err, bad)
			assert.ErrorIs(t, err, api.ErrInput, bad)
			assert.Contains(t, err.Error(), "invalid base URL", bad)
		}
	})

	t.Run("network failure", func(t *testing.T) {
		// A server that is already closed refuses the connection, which is
		// the closest offline stand-in for a network failure.
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
		srv.Close()
		a := New(WithBaseURL(srv.URL))
		_, err := a.Verify(context.Background(), tid)
		require.Error(t, err)
		assert.ErrorIs(t, err, api.ErrNetwork)
	})
}

func TestNew(t *testing.T) {
	t.Run("default timeout", func(t *testing.T) {
		a := New()
		assert.Equal(t, DefaultTimeout, a.HTTPClient().Timeout)
	})

	t.Run("WithTimeout overrides the default", func(t *testing.T) {
		a := New(WithTimeout(3 * time.Second))
		assert.Equal(t, 3*time.Second, a.HTTPClient().Timeout)
	})

	t.Run("HTTPClient exposes the underlying client", func(t *testing.T) {
		a := New()
		require.NotNil(t, a.HTTPClient())
	})
}

func TestVerifier(t *testing.T) {
	id := api.Identifier{Path: api.PathTaxID, Country: "DE", Code: "282741168"}

	t.Run("source", func(t *testing.T) {
		assert.Equal(t, Source, New().Source())
	})

	t.Run("supports EU tax identities only", func(t *testing.T) {
		a := New()
		assert.True(t, a.Supports(api.Identifier{Country: "ES", Code: "B85905495"}))
		assert.True(t, a.Supports(api.Identifier{Country: "EL", Code: "123456789"}), "VIES uses EL for Greece")
		assert.True(t, a.Supports(api.Identifier{Country: "XI", Code: "123456789"}), "VIES covers Northern Ireland as XI")
		assert.False(t, a.Supports(api.Identifier{Country: "GB", Code: "123456789"}), "GB proper is not covered")
		assert.False(t, a.Supports(api.Identifier{Country: "US", Code: "123456789"}))
		assert.False(t, a.Supports(api.Identifier{Code: "123456789"}))
		assert.False(t, a.Supports(api.Identifier{Country: "DE", Type: "HRB", Code: "12345"}), "org identities are not tax identities")
		assert.False(t, a.Supports(api.Identifier{Country: "DE", Key: "other", Code: "12345"}))
	})

	t.Run("valid with disclosed name", func(t *testing.T) {
		a := serve(t, http.StatusOK, viesValidNamedBody, nil)
		before := time.Now().Add(-time.Second)
		check, err := a.Verify(context.Background(), id)
		require.NoError(t, err)
		assert.Equal(t, api.PathTaxID, check.Path)
		assert.Equal(t, api.StatusValid, check.Status)
		assert.Equal(t, Source, check.Source)
		assert.True(t, check.CheckedAt.After(before))
		require.NotNil(t, check.Record)
		assert.Equal(t, "ACME GMBH", check.Record.Name)
		require.NotNil(t, check.TaxID)
		assert.Equal(t, "DE", check.TaxID.Country.String())
		assert.Equal(t, "282741168", check.TaxID.Code.String())
		assert.Empty(t, check.Mismatches, "verifiers do not compute mismatches")
	})

	t.Run("valid with masked name has no record", func(t *testing.T) {
		a := serve(t, http.StatusOK, viesValidBody, nil)
		check, err := a.Verify(context.Background(), id)
		require.NoError(t, err)
		assert.Equal(t, api.StatusValid, check.Status)
		assert.Nil(t, check.Record)
	})

	t.Run("invalid number is a check, not an error", func(t *testing.T) {
		a := serve(t, http.StatusOK, viesInvalidBody, nil)
		check, err := a.Verify(context.Background(), id)
		require.NoError(t, err)
		assert.Equal(t, api.StatusInvalid, check.Status)
		assert.Nil(t, check.Record)
	})

	t.Run("register failures are errors", func(t *testing.T) {
		a := serve(t, http.StatusInternalServerError, `{"message": "MS_UNAVAILABLE"}`, nil)
		check, err := a.Verify(context.Background(), id)
		require.Error(t, err)
		assert.Nil(t, check)
		assert.ErrorIs(t, err, api.ErrServer)
	})

	t.Run("bad request is an input error", func(t *testing.T) {
		a := serve(t, http.StatusBadRequest, `{"message": "Invalid VAT number format"}`, nil)
		_, err := a.Verify(context.Background(), id)
		assert.ErrorIs(t, err, api.ErrInput)
	})

	t.Run("rate limited", func(t *testing.T) {
		a := serve(t, http.StatusTooManyRequests, `{}`, http.Header{"Retry-After": {"30"}})
		_, err := a.Verify(context.Background(), id)
		var rl *api.RateLimitedError
		require.True(t, errors.As(err, &rl))
		assert.Equal(t, 30*time.Second, rl.RetryAfter)
	})

	t.Run("incomplete identifier is an input error", func(t *testing.T) {
		a := serve(t, http.StatusOK, viesValidBody, nil)
		for name, bad := range map[string]api.Identifier{
			"empty country": {Code: "B85905495"},
			"empty code":    {Country: "ES"},
		} {
			_, err := a.Verify(context.Background(), bad)
			assert.ErrorIs(t, err, api.ErrInput, name)
		}
	})
}
