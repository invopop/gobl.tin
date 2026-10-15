package dgfip

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl.tin/vies"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const danone = "27552032534"

// danoneCompany is the shape of the Annuaire des Entreprises answer for
// Danone, as it was on 2026-10-15, cut to the fields the verifier reads.
const danoneCompany = `{"results":[{"siren":"552032534","nom_complet":"DANONE","nom_raison_sociale":"DANONE","etat_administratif":"A","complements":{"est_entrepreneur_individuel":false},
"siege":{"numero_voie":"59","indice_repetition":null,"type_voie":"RUE","libelle_voie":"LA FAYETTE","complement_adresse":null,"distribution_speciale":null,"code_postal":"75009","libelle_commune":"PARIS","cedex":null,"libelle_cedex":null,"code_pays_etranger":null}}]}`

const inDataset = `{"total_count":1,"results":[{"vat_no":"27552032534"}]}`
const notInDataset = `{"total_count":0,"results":[]}`

// fakeLive is the live check of a number the dataset does not hold.
type fakeLive struct {
	ans   *tin.Answer
	err   error
	calls atomic.Int32
}

func (f *fakeLive) Source() cbc.Key              { return vies.Source }
func (f *fakeLive) Supports(tin.Identifier) bool { return true }
func (f *fakeLive) Verify(context.Context, tin.Request) (*tin.Answer, error) {
	f.calls.Add(1)
	return f.ans, f.err
}

type reply struct {
	status int
	body   string
	header http.Header
}

// setup serves the dataset and the company search with fixed replies.
func setup(t *testing.T, dataset, companies reply, live *fakeLive) (*Verifier, *atomic.Int32, *string) {
	t.Helper()
	var companyCalls atomic.Int32
	var query string
	serve := func(r reply, onCall func(*http.Request)) *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			onCall(req)
			for k, vs := range r.header {
				for _, v := range vs {
					w.Header().Add(k, v)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(r.status)
			_, _ = w.Write([]byte(r.body))
		}))
		t.Cleanup(srv.Close)
		return srv
	}
	ds := serve(dataset, func(r *http.Request) { query = r.URL.Query().Get("where") })
	cs := serve(companies, func(*http.Request) { companyCalls.Add(1) })
	if live == nil {
		live = &fakeLive{err: errors.New("live check not expected")}
	}
	v := New(WithDatasetURL(ds.URL), WithCompaniesURL(cs.URL), WithLive(live))
	return v, &companyCalls, &query
}

func request(code cbc.Code) tin.Request {
	return tin.Request{Identifier: tin.Identifier{Path: tin.PathTaxID, Country: "FR", Code: code}}
}

func TestVerify(t *testing.T) {
	ok := func(body string) reply { return reply{status: http.StatusOK, body: body} }

	t.Run("a number in the dataset is valid, with the name and address of the company", func(t *testing.T) {
		v, _, query := setup(t, ok(inDataset), ok(danoneCompany), nil)
		ans, err := v.Verify(context.Background(), request(danone))
		require.NoError(t, err)
		assert.Equal(t, `vat_no="27552032534"`, *query)
		assert.Equal(t, tin.StatusValid, ans.Status)
		assert.Equal(t, "FR", ans.TaxID.Country.String())
		assert.Equal(t, cbc.Code(danone), ans.TaxID.Code)
		assert.Equal(t, &tin.Record{
			Name:    "DANONE",
			Address: &org.Address{Number: "59", Street: "RUE LA FAYETTE", Code: "75009", Locality: "PARIS", Country: "FR"},
		}, ans.Record)
	})

	t.Run("a sole trader is named after the person", func(t *testing.T) {
		body := `{"results":[{"siren":"390435220","nom_complet":"FRANCK DELHAYE (FRANCK DELHAYE)","nom_raison_sociale":null,"etat_administratif":"A","complements":{"est_entrepreneur_individuel":true},
"siege":{"numero_voie":"2773","type_voie":"ROUTE","libelle_voie":"DE CORNUS","code_postal":"46330","libelle_commune":"CENEVIERES"}}]}`
		v, _, _ := setup(t, ok(inDataset), ok(body), nil)
		ans, err := v.Verify(context.Background(), request("0A390435220"))
		require.NoError(t, err)
		assert.Equal(t, "FRANCK DELHAYE", ans.Record.Name)
		assert.Equal(t, &org.Address{Number: "2773", Street: "ROUTE DE CORNUS", Code: "46330", Locality: "CENEVIERES", Country: "FR"}, ans.Record.Address)
	})

	t.Run("a person who opted out of publication gets no name or address", func(t *testing.T) {
		body := `{"results":[{"siren":"519341283","nom_complet":"[NON-DIFFUSIBLE]","nom_raison_sociale":"[NON-DIFFUSIBLE]","etat_administratif":"A","complements":{"est_entrepreneur_individuel":true},
"siege":{"numero_voie":"[NON-DIFFUSIBLE]","type_voie":"[NON-DIFFUSIBLE]","libelle_voie":"[NON-DIFFUSIBLE]","code_postal":"[NON-DIFFUSIBLE]","libelle_commune":"NIMES"}}]}`
		v, _, _ := setup(t, ok(inDataset), ok(body), nil)
		ans, err := v.Verify(context.Background(), request("0A519341283"))
		require.NoError(t, err)
		assert.Equal(t, tin.StatusValid, ans.Status)
		assert.Nil(t, ans.Record)
	})

	t.Run("a company that the directory does not list gives no record", func(t *testing.T) {
		v, _, _ := setup(t, ok(inDataset), ok(`{"results":[{"siren":"999999999","nom_raison_sociale":"OTHER"}]}`), nil)
		ans, err := v.Verify(context.Background(), request(danone))
		require.NoError(t, err)
		assert.Equal(t, tin.StatusValid, ans.Status)
		assert.Nil(t, ans.Record)
	})

	t.Run("a closed company is inactive", func(t *testing.T) {
		body := `{"results":[{"siren":"552032534","nom_raison_sociale":"DANONE","etat_administratif":"C","siege":{}}]}`
		v, _, _ := setup(t, ok(inDataset), ok(body), nil)
		ans, err := v.Verify(context.Background(), request(danone))
		require.NoError(t, err)
		assert.Equal(t, &tin.Record{Name: "DANONE", Status: tin.RecordStatusInactive}, ans.Record)
	})

	t.Run("a CEDEX replaces the postcode and the locality", func(t *testing.T) {
		body := `{"results":[{"siren":"552032534","nom_raison_sociale":"DANONE","etat_administratif":"A",
"siege":{"numero_voie":"1","indice_repetition":"B","type_voie":"AVENUE","libelle_voie":"DE PARIS","complement_adresse":"BATIMENT A","distribution_speciale":"BP 12","code_postal":"75009","libelle_commune":"PARIS","cedex":"75441","libelle_cedex":"PARIS CEDEX 09"}}]}`
		v, _, _ := setup(t, ok(inDataset), ok(body), nil)
		ans, err := v.Verify(context.Background(), request(danone))
		require.NoError(t, err)
		assert.Equal(t, &org.Address{Number: "1B", Street: "AVENUE DE PARIS", StreetExtra: "BATIMENT A", PostOfficeBox: "BP 12", Code: "75441", Locality: "PARIS CEDEX 09", Country: "FR"}, ans.Record.Address)
	})

	t.Run("a head office abroad gives no address", func(t *testing.T) {
		body := `{"results":[{"siren":"552032534","nom_raison_sociale":"DANONE","etat_administratif":"A","siege":{"libelle_voie":"MAIN STREET","code_postal":"","code_pays_etranger":"99132"}}]}`
		v, _, _ := setup(t, ok(inDataset), ok(body), nil)
		ans, err := v.Verify(context.Background(), request(danone))
		require.NoError(t, err)
		assert.Equal(t, &tin.Record{Name: "DANONE"}, ans.Record)
	})

	t.Run("a number the dataset does not hold is valid when the live check confirms it", func(t *testing.T) {
		live := &fakeLive{ans: &tin.Answer{Status: tin.StatusValid, Record: &tin.Record{Name: "SA DANONE"}}}
		v, companies, _ := setup(t, ok(notInDataset), ok(danoneCompany), live)
		ans, err := v.Verify(context.Background(), request(danone))
		require.NoError(t, err)
		assert.Equal(t, int32(1), live.calls.Load())
		assert.Equal(t, int32(1), companies.Load())
		assert.Equal(t, tin.StatusValid, ans.Status)
		assert.Equal(t, "DANONE", ans.Record.Name, "the name comes from the directory")
	})

	t.Run("a number the dataset does not hold is invalid when the live check rejects it", func(t *testing.T) {
		live := &fakeLive{ans: &tin.Answer{Status: tin.StatusInvalid}}
		v, companies, _ := setup(t, ok(notInDataset), ok(danoneCompany), live)
		ans, err := v.Verify(context.Background(), request(danone))
		require.NoError(t, err)
		assert.Equal(t, tin.StatusInvalid, ans.Status)
		assert.Nil(t, ans.Record)
		assert.Equal(t, int32(0), companies.Load(), "an invalid number gets no company query")
	})

	t.Run("a failed live check is an error", func(t *testing.T) {
		live := &fakeLive{err: &tin.RateLimitedError{RetryAfter: time.Minute}}
		v, _, _ := setup(t, ok(notInDataset), ok(danoneCompany), live)
		_, err := v.Verify(context.Background(), request(danone))
		var rl *tin.RateLimitedError
		assert.ErrorAs(t, err, &rl)
	})

	t.Run("a rate limited dataset waits until the quota resets", func(t *testing.T) {
		reset := time.Now().UTC().Add(2 * time.Hour).Format("2006-01-02 15:04:05-07:00")
		v, _, _ := setup(t, reply{status: http.StatusTooManyRequests, body: `{}`, header: http.Header{"X-Ratelimit-Reset": {reset}}}, ok(danoneCompany), nil)
		_, err := v.Verify(context.Background(), request(danone))
		var rl *tin.RateLimitedError
		require.ErrorAs(t, err, &rl)
		assert.InDelta(t, (2 * time.Hour).Seconds(), rl.RetryAfter.Seconds(), 5)
	})

	t.Run("a failing dataset is a server error", func(t *testing.T) {
		v, _, _ := setup(t, reply{status: http.StatusBadGateway, body: "bad gateway"}, ok(danoneCompany), nil)
		_, err := v.Verify(context.Background(), request(danone))
		require.ErrorIs(t, err, tin.ErrServer)
		var te *tin.Error
		require.ErrorAs(t, err, &te)
		assert.Equal(t, "502", te.Code())
	})

	t.Run("an unreadable dataset answer is a network error", func(t *testing.T) {
		v, _, _ := setup(t, ok(`{"results":[]}`), ok(danoneCompany), nil)
		_, err := v.Verify(context.Background(), request(danone))
		assert.ErrorIs(t, err, tin.ErrNetwork)
	})

	t.Run("a rate limited company search is a rate limit", func(t *testing.T) {
		v, _, _ := setup(t, ok(inDataset), reply{status: http.StatusTooManyRequests, body: `{}`, header: http.Header{"Retry-After": {"7"}}}, nil)
		_, err := v.Verify(context.Background(), request(danone))
		var rl *tin.RateLimitedError
		require.ErrorAs(t, err, &rl)
		assert.Equal(t, 7*time.Second, rl.RetryAfter)
	})

	t.Run("a failing company search is a server error", func(t *testing.T) {
		v, _, _ := setup(t, ok(inDataset), reply{status: http.StatusServiceUnavailable, body: "down"}, nil)
		_, err := v.Verify(context.Background(), request(danone))
		assert.ErrorIs(t, err, tin.ErrServer)
	})

	t.Run("a malformed number is an input error, without a request", func(t *testing.T) {
		v, companies, query := setup(t, ok(inDataset), ok(danoneCompany), nil)
		for _, code := range []cbc.Code{"", "552032534", `27552032534" OR "1"="1`} {
			_, err := v.Verify(context.Background(), request(code))
			assert.ErrorIs(t, err, tin.ErrInput, code)
		}
		assert.Empty(t, *query)
		assert.Equal(t, int32(0), companies.Load())
	})
}

func TestVerifierRouting(t *testing.T) {
	v := New()
	assert.Equal(t, Source, v.Source())
	assert.True(t, v.Supports(tin.Identifier{Country: "FR", Code: danone}))
	assert.False(t, v.Supports(tin.Identifier{Country: "ES", Code: "B85905495"}))
	assert.Equal(t, vies.Source, v.live.Source(), "the live check is VIES by default")
}

func TestNewInvalidBaseURL(t *testing.T) {
	for _, opt := range []Option{WithDatasetURL("://bad"), WithCompaniesURL("ftp://example.com")} {
		_, err := New(opt).Verify(context.Background(), request(danone))
		assert.ErrorIs(t, err, tin.ErrServer)
	}
}

// TestClientFallsBackToVIES checks that a DGFiP failure lets the next
// verifier answer, as Lookup orders them.
func TestClientFallsBackToVIES(t *testing.T) {
	v, _, _ := setup(t, reply{status: http.StatusBadGateway, body: "down"}, reply{status: http.StatusOK, body: danoneCompany}, nil)
	next := &fakeLive{ans: &tin.Answer{Status: tin.StatusValid, Record: &tin.Record{Name: "SA DANONE"}}}
	check, err := tin.New(v, next).VerifyTaxID(context.Background(), &taxIdentity)
	require.NoError(t, err)
	assert.Equal(t, tin.StatusValid, check.Status)
	assert.Equal(t, vies.Source, check.Source)
}

var taxIdentity = *taxID(danone)
