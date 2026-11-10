package dgfip

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl.tin/vies"
	"github.com/invopop/gobl/cbc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const danone = "27552032534"

const inDataset = `{"total_count":1,"results":[{"vat_no":"27552032534"}]}`
const notInDataset = `{"total_count":0,"results":[]}`

// fakeVIES stands in for VIES, the next verifier.
type fakeVIES struct {
	ans   *tin.Answer
	err   error
	calls atomic.Int32
}

func (f *fakeVIES) Source() cbc.Key              { return vies.Source }
func (f *fakeVIES) Supports(tin.Identifier) bool { return true }
func (f *fakeVIES) Verify(context.Context, tin.Request) (*tin.Answer, error) {
	f.calls.Add(1)
	return f.ans, f.err
}

type reply struct {
	status int
	body   string
	header http.Header
}

// setup serves the dataset with a fixed reply.
func setup(t *testing.T, dataset reply) (*Verifier, *atomic.Int32, *string) {
	t.Helper()
	var calls atomic.Int32
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		query = req.URL.Query().Get("where")
		for k, vs := range dataset.header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(dataset.status)
		_, _ = w.Write([]byte(dataset.body))
	}))
	t.Cleanup(srv.Close)
	return New(WithDatasetURL(srv.URL)), &calls, &query
}

func request(code cbc.Code) tin.Request {
	return tin.Request{Identifier: tin.Identifier{Path: tin.PathTaxID, Country: "FR", Code: code}}
}

func TestVerify(t *testing.T) {
	ok := func(body string) reply { return reply{status: http.StatusOK, body: body} }

	t.Run("a number in the dataset is valid, without a record", func(t *testing.T) {
		v, calls, query := setup(t, ok(inDataset))
		ans, err := v.Verify(context.Background(), request(danone))
		require.NoError(t, err)
		assert.Equal(t, `vat_no="27552032534"`, *query)
		assert.Equal(t, tin.StatusValid, ans.Status)
		assert.Equal(t, "FR", ans.TaxID.Country.String())
		assert.Equal(t, cbc.Code(danone), ans.TaxID.Code)
		assert.Nil(t, ans.Record)
		assert.Empty(t, ans.Reason)
		assert.Equal(t, int32(1), calls.Load())
	})

	t.Run("a number the dataset does not hold is invalid, also with the key of its SIREN", func(t *testing.T) {
		// 42 is the key that GOBL computes from the SIREN 904320728.
		for _, code := range []cbc.Code{danone, "42904320728", "ZZ552032534"} {
			v, calls, _ := setup(t, ok(notInDataset))
			ans, err := v.Verify(context.Background(), request(code))
			require.NoError(t, err)
			assert.Equal(t, tin.StatusInvalid, ans.Status, code)
			assert.Equal(t, code, ans.TaxID.Code, code)
			assert.Nil(t, ans.Record, code)
			assert.Equal(t, reasonNotInDataset, ans.Reason, code)
			assert.Equal(t, int32(1), calls.Load(), code)
		}
	})

	t.Run("a rate limited dataset waits until the quota resets", func(t *testing.T) {
		reset := time.Now().UTC().Add(2 * time.Hour).Format("2006-01-02 15:04:05-07:00")
		v, _, _ := setup(t, reply{status: http.StatusTooManyRequests, body: `{}`, header: http.Header{"X-Ratelimit-Reset": {reset}}})
		_, err := v.Verify(context.Background(), request(danone))
		var rl *tin.RateLimitedError
		require.ErrorAs(t, err, &rl)
		assert.InDelta(t, (2 * time.Hour).Seconds(), rl.RetryAfter.Seconds(), 5)
	})

	t.Run("a rate limited dataset with Retry-After waits that long", func(t *testing.T) {
		v, _, _ := setup(t, reply{status: http.StatusTooManyRequests, body: `{}`, header: http.Header{"Retry-After": {"7"}}})
		_, err := v.Verify(context.Background(), request(danone))
		var rl *tin.RateLimitedError
		require.ErrorAs(t, err, &rl)
		assert.Equal(t, 7*time.Second, rl.RetryAfter)
	})

	t.Run("a failing dataset is a server error", func(t *testing.T) {
		v, _, _ := setup(t, reply{status: http.StatusBadGateway, body: "bad gateway"})
		_, err := v.Verify(context.Background(), request(danone))
		require.ErrorIs(t, err, tin.ErrServer)
		var te *tin.Error
		require.ErrorAs(t, err, &te)
		assert.Equal(t, "502", te.Code())
	})

	t.Run("an unreadable dataset answer is a network error", func(t *testing.T) {
		v, _, _ := setup(t, ok(`{"results":[]}`))
		_, err := v.Verify(context.Background(), request(danone))
		assert.ErrorIs(t, err, tin.ErrNetwork)
	})

	t.Run("a malformed number is an input error, without a request", func(t *testing.T) {
		v, calls, query := setup(t, ok(inDataset))
		for _, code := range []cbc.Code{"", "552032534", `27552032534" OR "1"="1`} {
			_, err := v.Verify(context.Background(), request(code))
			assert.ErrorIs(t, err, tin.ErrInput, code)
		}
		assert.Empty(t, *query)
		assert.Equal(t, int32(0), calls.Load())
	})
}

func TestVerifierRouting(t *testing.T) {
	v := New()
	assert.Equal(t, Source, v.Source())
	assert.True(t, v.Supports(tin.Identifier{Country: "FR", Code: danone}))
	assert.False(t, v.Supports(tin.Identifier{Country: "ES", Code: "B85905495"}))
}

func TestNewInvalidBaseURL(t *testing.T) {
	for _, opt := range []Option{WithDatasetURL("://bad"), WithDatasetURL("ftp://example.com")} {
		_, err := New(opt).Verify(context.Background(), request(danone))
		assert.ErrorIs(t, err, tin.ErrServer)
	}
}

// TestClientNeverAsksVIES checks that a DGFiP failure is final, as Lookup
// orders the verifiers: VIES is never asked for France.
func TestClientNeverAsksVIES(t *testing.T) {
	v, _, _ := setup(t, reply{status: http.StatusBadGateway, body: "down"})
	next := &fakeVIES{ans: &tin.Answer{Status: tin.StatusValid, Record: &tin.Record{Name: "SA DANONE"}}}
	check, err := tin.New(v, next).VerifyTaxID(context.Background(), &taxIdentity)
	require.NoError(t, err)
	assert.Equal(t, tin.StatusUnverified, check.Status)
	assert.Equal(t, Source, check.Source)
	assert.Equal(t, int32(0), next.calls.Load())
}

var taxIdentity = *taxID(danone)
