package hmrc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// found is a lookup response in the shape of the example in HMRC's API
// specification, with HMRC's mock company.
const found = `{"target":{"name":"Credite Sberger Donal Inc.","vatNumber":"553557881","address":{"line1":"131B Barton Hamlet","postcode":"SW97 5CK","countryCode":"GB"}},"processingDate":"2026-10-13T10:49:29+01:00"}`

// notFound is the lookup response for a VAT number that HMRC does not hold.
const notFound = `{"code":"NOT_FOUND","message":"targetVrn does not match a registered company"}`

// fakeHMRC is a test server with the token and lookup endpoints. Each
// lookup takes the next response from the queue; the last one repeats.
type fakeHMRC struct {
	mu        sync.Mutex
	tokens    int
	tokenForm map[string]string
	lookups   []*http.Request
	replies   []reply

	// tokenStatus, when set, makes the token endpoint fail with it.
	tokenStatus int
}

// reply is one lookup response.
type reply struct {
	status int
	body   string
	header http.Header
}

func (f *fakeHMRC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == tokenPath {
		f.tokens++
		_ = r.ParseForm()
		f.tokenForm = map[string]string{}
		for k := range r.PostForm {
			f.tokenForm[k] = r.PostForm.Get(k)
		}
		if f.tokenStatus != 0 {
			w.WriteHeader(f.tokenStatus)
			_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"invalid client id or secret"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"token-` + string(rune('0'+f.tokens)) + `","scope":"read:vat","expires_in":14400,"token_type":"bearer"}`))
		return
	}
	f.lookups = append(f.lookups, r)
	rep := f.replies[0]
	if len(f.replies) > 1 {
		f.replies = f.replies[1:]
	}
	for k, vs := range rep.header {
		w.Header()[k] = vs
	}
	w.WriteHeader(rep.status)
	_, _ = w.Write([]byte(rep.body))
}

// serve starts a fake HMRC server with the lookup replies and returns a
// verifier pointed at it.
func serve(t *testing.T, replies ...reply) (*Verifier, *fakeHMRC) {
	t.Helper()
	f := &fakeHMRC{replies: replies}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return New("client-id", "client-secret", WithBaseURL(srv.URL)), f
}

var vrn = tin.Identifier{Path: tin.PathTaxID, Country: "GB", Code: "553557881"}

func TestNew(t *testing.T) {
	t.Run("default base URL and timeout", func(t *testing.T) {
		v := New("id", "secret")
		require.NoError(t, v.initErr)
		assert.Equal(t, DefaultBaseURL, v.baseURL)
		assert.Equal(t, DefaultTimeout, v.conn.GetClient().Timeout)
	})

	t.Run("WithSandbox uses the sandbox", func(t *testing.T) {
		assert.Equal(t, SandboxBaseURL, New("id", "secret", WithSandbox()).baseURL)
	})

	t.Run("WithTimeout overrides the default", func(t *testing.T) {
		assert.Equal(t, 3*time.Second, New("id", "secret", WithTimeout(3*time.Second)).conn.GetClient().Timeout)
	})

	t.Run("WithHTTPClient leaves the caller's client as it was", func(t *testing.T) {
		hc := &http.Client{Timeout: time.Hour}
		New("id", "secret", WithHTTPClient(hc))
		assert.Equal(t, time.Hour, hc.Timeout)
	})

	t.Run("missing credentials fail on the first call, without a request", func(t *testing.T) {
		for _, c := range [][2]string{{"", "secret"}, {"id", ""}} {
			srv, f := serve(t, reply{status: http.StatusOK, body: found})
			v := New(c[0], c[1], WithBaseURL(srv.baseURL))
			_, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
			assert.ErrorIs(t, err, tin.ErrServer)
			assert.Contains(t, err.Error(), "no client credentials")
			assert.Zero(t, f.tokens)
		}
	})

	t.Run("an invalid base URL fails on the first call", func(t *testing.T) {
		for _, u := range []string{"://bad", "ftp://example.com", "https://"} {
			_, err := New("id", "secret", WithBaseURL(u)).Verify(context.Background(), tin.Request{Identifier: vrn})
			assert.ErrorIs(t, err, tin.ErrInput, u)
		}
	})

	t.Run("implements tin.Verifier", func(*testing.T) {
		var _ tin.Verifier = New("id", "secret")
	})
}

func TestVerifier(t *testing.T) {
	t.Run("source", func(t *testing.T) {
		assert.Equal(t, Source, New("id", "secret").Source())
	})

	t.Run("supports UK VAT numbers of 9 or 12 digits only", func(t *testing.T) {
		v := New("id", "secret")
		assert.True(t, v.Supports(vrn))
		assert.True(t, v.Supports(tin.Identifier{Country: "GB", Code: "146295999727"}))
		assert.False(t, v.Supports(tin.Identifier{Country: "GB", Code: "GD001"}), "HMRC does not check government departments")
		assert.False(t, v.Supports(tin.Identifier{Country: "GB", Code: "HA500"}), "HMRC does not check health authorities")
		assert.False(t, v.Supports(tin.Identifier{Country: "XI", Code: "553557881"}), "Northern Ireland is in VIES")
		assert.False(t, v.Supports(tin.Identifier{Country: "IE", Code: "553557881"}))
	})

	t.Run("gets a token and looks up the number", func(t *testing.T) {
		v, f := serve(t, reply{status: http.StatusOK, body: found})
		_, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"client_id":     "client-id",
			"client_secret": "client-secret",
			"grant_type":    "client_credentials",
			"scope":         "read:vat",
		}, f.tokenForm)
		require.Len(t, f.lookups, 1)
		r := f.lookups[0]
		assert.Equal(t, "/organisations/vat/check-vat-number/lookup/553557881", r.URL.Path)
		assert.Equal(t, acceptV2, r.Header.Get("Accept"))
		assert.Equal(t, "Bearer token-1", r.Header.Get("Authorization"))
		assert.Equal(t, userAgent, r.Header.Get("User-Agent"))
	})

	t.Run("a registered number is valid with its name and address", func(t *testing.T) {
		v, _ := serve(t, reply{status: http.StatusOK, body: found})
		ans, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		require.NoError(t, err)
		assert.Equal(t, tin.StatusValid, ans.Status)
		assert.Equal(t, &tax.Identity{Country: "GB", Code: "553557881"}, ans.TaxID)
		assert.Equal(t, time.Date(2026, 10, 13, 9, 49, 29, 0, time.UTC), ans.CheckedAt)
		assert.Equal(t, &tin.Record{
			Name:    "Credite Sberger Donal Inc.",
			Address: &org.Address{Street: "131B Barton Hamlet", Code: "SW97 5CK", Country: "GB"},
		}, ans.Record)
	})

	t.Run("the token is reused until it expires", func(t *testing.T) {
		v, f := serve(t, reply{status: http.StatusOK, body: found})
		for range 3 {
			_, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
			require.NoError(t, err)
		}
		assert.Equal(t, 1, f.tokens)

		v.expires = time.Now().Add(-time.Second)
		_, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		require.NoError(t, err)
		assert.Equal(t, 2, f.tokens)
	})

	t.Run("a rejected token is renewed once", func(t *testing.T) {
		v, f := serve(t,
			reply{status: http.StatusUnauthorized, body: `{"code":"INVALID_CREDENTIALS","message":"Invalid Authentication information provided"}`},
			reply{status: http.StatusOK, body: found},
		)
		ans, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		require.NoError(t, err)
		assert.Equal(t, tin.StatusValid, ans.Status)
		assert.Equal(t, 2, f.tokens)
		require.Len(t, f.lookups, 2)
		assert.Equal(t, "Bearer token-2", f.lookups[1].Header.Get("Authorization"))
	})

	t.Run("a token rejected twice is a server error", func(t *testing.T) {
		v, f := serve(t, reply{status: http.StatusUnauthorized, body: `{"code":"INVALID_CREDENTIALS","message":"Invalid Authentication information provided"}`})
		_, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		assert.ErrorIs(t, err, tin.ErrServer)
		assert.Contains(t, err.Error(), "INVALID_CREDENTIALS")
		assert.Len(t, f.lookups, 2)
	})

	t.Run("wrong credentials are a server error", func(t *testing.T) {
		v, f := serve(t, reply{status: http.StatusOK, body: found})
		f.tokenStatus = http.StatusUnauthorized
		_, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		assert.ErrorIs(t, err, tin.ErrServer)
		assert.Contains(t, err.Error(), "getting a token")
		assert.Empty(t, f.lookups)
	})

	t.Run("a rate limited token request", func(t *testing.T) {
		v, f := serve(t, reply{status: http.StatusOK, body: found})
		f.tokenStatus = http.StatusTooManyRequests
		_, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		var rl *tin.RateLimitedError
		require.ErrorAs(t, err, &rl)
	})

	t.Run("a number HMRC does not hold is invalid", func(t *testing.T) {
		v, _ := serve(t, reply{status: http.StatusNotFound, body: notFound})
		ans, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		require.NoError(t, err)
		assert.Equal(t, tin.StatusInvalid, ans.Status)
		assert.Equal(t, &tax.Identity{Country: "GB", Code: "553557881"}, ans.TaxID)
		assert.Nil(t, ans.Record)
	})

	t.Run("another 404 is a server error", func(t *testing.T) {
		v, _ := serve(t, reply{status: http.StatusNotFound, body: `{"code":"MATCHING_RESOURCE_NOT_FOUND","message":"A resource with the name in the request can not be found in the API"}`})
		_, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		assert.ErrorIs(t, err, tin.ErrServer)
		assert.Contains(t, err.Error(), "MATCHING_RESOURCE_NOT_FOUND")
	})

	t.Run("a code HMRC does not check is an input error, without a request", func(t *testing.T) {
		v, f := serve(t, reply{status: http.StatusOK, body: found})
		_, err := v.Verify(context.Background(), tin.Request{Identifier: tin.Identifier{Country: "GB", Code: "GD001"}})
		assert.ErrorIs(t, err, tin.ErrInput)
		assert.Zero(t, f.tokens)
	})

	t.Run("a bad request is an input error", func(t *testing.T) {
		v, _ := serve(t, reply{status: http.StatusBadRequest, body: `{"code":"INVALID_REQUEST","message":"Invalid targetVrn - Vrn parameters should be 9 or 12 digits"}`})
		_, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		assert.ErrorIs(t, err, tin.ErrInput)
		assert.Contains(t, err.Error(), "INVALID_REQUEST: Invalid targetVrn")
	})

	t.Run("a forbidden application is a server error", func(t *testing.T) {
		v, _ := serve(t, reply{status: http.StatusForbidden, body: `{"code":"RESOURCE_FORBIDDEN","message":"The application is not subscribed to the API"}`})
		_, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		assert.ErrorIs(t, err, tin.ErrServer)
	})

	t.Run("a server error", func(t *testing.T) {
		v, _ := serve(t, reply{status: http.StatusInternalServerError, body: `{"code":"INTERNAL_SERVER_ERROR","message":"Unknown error"}`})
		_, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		assert.ErrorIs(t, err, tin.ErrServer)
	})

	t.Run("rate limited", func(t *testing.T) {
		v, _ := serve(t, reply{status: http.StatusTooManyRequests, body: `{"code":"MESSAGE_THROTTLED_OUT","message":"The request for the API is throttled as you have exceeded your quota."}`, header: http.Header{"Retry-After": {"30"}}})
		_, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		var rl *tin.RateLimitedError
		require.ErrorAs(t, err, &rl)
		assert.Equal(t, 30*time.Second, rl.RetryAfter)
	})

	t.Run("rate limited without Retry-After uses the default", func(t *testing.T) {
		v, _ := serve(t, reply{status: http.StatusTooManyRequests, body: `{}`})
		_, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		var rl *tin.RateLimitedError
		require.ErrorAs(t, err, &rl)
		assert.Equal(t, defaultRetryAfter, rl.RetryAfter)
	})

	t.Run("an unreadable body is a network error", func(t *testing.T) {
		v, _ := serve(t, reply{status: http.StatusOK, body: `{`})
		_, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		assert.ErrorIs(t, err, tin.ErrNetwork)
	})

	t.Run("a body without a target is a network error", func(t *testing.T) {
		v, _ := serve(t, reply{status: http.StatusOK, body: `{"processingDate":"2026-10-13T10:49:29+01:00"}`})
		_, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		assert.ErrorIs(t, err, tin.ErrNetwork)
	})

	t.Run("a dial failure is a network error", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		srv.Close()
		_, err := New("id", "secret", WithBaseURL(srv.URL)).Verify(context.Background(), tin.Request{Identifier: vrn})
		assert.ErrorIs(t, err, tin.ErrNetwork)
	})

	t.Run("an empty echo falls back to the request", func(t *testing.T) {
		v, _ := serve(t, reply{status: http.StatusOK, body: `{"target":{"name":"A LTD","vatNumber":"","address":{"line1":"1 HIGH STREET","countryCode":"GB"}},"processingDate":"bad"}`})
		ans, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		require.NoError(t, err)
		assert.Equal(t, &tax.Identity{Country: "GB", Code: "553557881"}, ans.TaxID)
		assert.WithinDuration(t, time.Now(), ans.CheckedAt, time.Minute, "an unreadable date falls back to now")
	})

	t.Run("messages are clamped to valid text", func(t *testing.T) {
		v, _ := serve(t, reply{status: http.StatusInternalServerError, body: strings.Repeat("é", 500)})
		_, err := v.Verify(context.Background(), tin.Request{Identifier: vrn})
		require.Error(t, err)
		assert.True(t, utf8.ValidString(err.Error()))
		assert.Less(t, len(err.Error()), 300)
	})

	t.Run("a cancelled context is a network error", func(t *testing.T) {
		v, _ := serve(t, reply{status: http.StatusOK, body: found})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := v.Verify(ctx, tin.Request{Identifier: vrn})
		assert.ErrorIs(t, err, tin.ErrNetwork)
		assert.True(t, errors.Is(err, context.Canceled))
	})
}

func TestAddress(t *testing.T) {
	tests := []struct {
		name string
		in   *address
		want *org.Address
	}{
		{"no address", nil, nil},
		{
			"one line",
			&address{Line1: "131B Barton Hamlet", Postcode: "SW97 5CK", CountryCode: "GB"},
			&org.Address{Street: "131B Barton Hamlet", Code: "SW97 5CK", Country: "GB"},
		},
		{
			"two lines: the last one is the post town",
			&address{Line1: "1 High Street", Line2: "Guildford", Postcode: "GU1 1AA", CountryCode: "GB"},
			&org.Address{Street: "1 High Street", Locality: "Guildford", Code: "GU1 1AA", Country: "GB"},
		},
		{
			"the lines between are the extra street part",
			&address{Line1: "Unit 4", Line2: "Riverside Park", Line3: " ", Line4: "Old Road", Line5: "Leeds", Postcode: "LS1 1AA", CountryCode: "gb"},
			&org.Address{Street: "Unit 4", StreetExtra: "Riverside Park, Old Road", Locality: "Leeds", Code: "LS1 1AA", Country: "GB"},
		},
		{
			"no street and no postcode is no address",
			&address{CountryCode: "GB"},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.in.toOrg())
		})
	}
}

func TestClient(t *testing.T) {
	t.Run("the registered name and address are patched", func(t *testing.T) {
		v, _ := serve(t, reply{status: http.StatusOK, body: found})
		party := &org.Party{Name: "Credite Sberger", TaxID: &tax.Identity{Country: "GB", Code: "553557881"}}
		report, err := tin.New(v).Verify(context.Background(), party)
		require.NoError(t, err)
		require.Len(t, report.Checks, 1)
		check := report.Checks[0]
		assert.Equal(t, tin.StatusValid, check.Status)
		assert.Equal(t, Source, check.Source)
		require.Len(t, check.Mismatches, 1)
		assert.Equal(t, tin.MismatchName, check.Mismatches[0].Field)
		patch, err := tin.Patch(party, report)
		require.NoError(t, err)
		assert.JSONEq(t, `{"name":"Credite Sberger Donal Inc.","addresses":[{"street":"131B Barton Hamlet","code":"SW97 5CK","country":"GB"}]}`, string(patch))
	})

	t.Run("a government department code is unsupported", func(t *testing.T) {
		check, err := tin.New(New("id", "secret")).VerifyTaxID(context.Background(), &tax.Identity{Country: "GB", Code: "GD001"})
		require.NoError(t, err)
		assert.Equal(t, tin.StatusUnsupported, check.Status)
	})
}
