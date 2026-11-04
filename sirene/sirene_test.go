package sirene

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const danone = "552032534"

// danoneCompany is the shape of the Annuaire des Entreprises answer for
// Danone, as it was on 2026-10-15, cut to the fields the verifier reads.
const danoneCompany = `{"results":[{"siren":"552032534","nom_complet":"DANONE","nom_raison_sociale":"DANONE","etat_administratif":"A","complements":{"est_entrepreneur_individuel":false},
"siege":{"numero_voie":"59","indice_repetition":null,"type_voie":"RUE","libelle_voie":"LA FAYETTE","complement_adresse":null,"distribution_speciale":null,"code_postal":"75009","libelle_commune":"PARIS","cedex":null,"libelle_cedex":null,"code_pays_etranger":null}}]}`

var danoneOffice = &org.Address{Number: "59", Street: "RUE LA FAYETTE", Code: "75009", Locality: "PARIS", Country: "FR"}

type reply struct {
	status int
	body   string
	header http.Header
}

func ok(body string) reply { return reply{status: http.StatusOK, body: body} }

// setup serves the company search with a fixed reply.
func setup(t *testing.T, r reply) (*Verifier, *atomic.Int32, *string) {
	t.Helper()
	var calls atomic.Int32
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		query = req.URL.Query().Get("q")
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
	return New(WithURL(srv.URL)), &calls, &query
}

func request(code cbc.Code) tin.Request {
	return tin.Request{Identifier: tin.Identifier{Path: "/identities/0", Type: IdentityType, Code: code}}
}

func TestVerify(t *testing.T) {
	ctx := context.Background()

	t.Run("a company in Sirene is valid, with its name and head office", func(t *testing.T) {
		v, _, query := setup(t, ok(danoneCompany))
		ans, err := v.Verify(ctx, request(danone))
		require.NoError(t, err)
		assert.Equal(t, danone, *query)
		assert.Equal(t, tin.StatusValid, ans.Status)
		assert.Nil(t, ans.TaxID)
		assert.Equal(t, &tin.Record{Name: "DANONE", Address: danoneOffice}, ans.Record)
	})

	t.Run("a closed company is dissolved", func(t *testing.T) {
		v, _, _ := setup(t, ok(`{"results":[{"siren":"552032534","nom_raison_sociale":"DANONE","etat_administratif":"C","siege":{}}]}`))
		ans, err := v.Verify(ctx, request(danone))
		require.NoError(t, err)
		assert.Equal(t, tin.StatusValid, ans.Status)
		assert.Equal(t, &tin.Record{Name: "DANONE", Status: tin.RecordStatusDissolved}, ans.Record)
	})

	t.Run("a SIREN that Sirene does not list is invalid", func(t *testing.T) {
		for _, body := range []string{`{"results":[]}`, `{"results":[{"siren":"999999999","nom_raison_sociale":"OTHER"}]}`} {
			v, _, _ := setup(t, ok(body))
			ans, err := v.Verify(ctx, request(danone))
			require.NoError(t, err)
			assert.Equal(t, tin.StatusInvalid, ans.Status, body)
			assert.Nil(t, ans.Record, body)
		}
	})

	t.Run("a sole trader is named after the person", func(t *testing.T) {
		body := `{"results":[{"siren":"390435220","nom_complet":"FRANCK DELHAYE (FRANCK DELHAYE)","nom_raison_sociale":null,"etat_administratif":"A","complements":{"est_entrepreneur_individuel":true},
"siege":{"numero_voie":"2773","type_voie":"ROUTE","libelle_voie":"DE CORNUS","code_postal":"46330","libelle_commune":"CENEVIERES"}}]}`
		v, _, _ := setup(t, ok(body))
		ans, err := v.Verify(ctx, request("390435220"))
		require.NoError(t, err)
		assert.Equal(t, &tin.Record{
			Name:    "FRANCK DELHAYE",
			Address: &org.Address{Number: "2773", Street: "ROUTE DE CORNUS", Code: "46330", Locality: "CENEVIERES", Country: "FR"},
		}, ans.Record)
	})

	t.Run("a person who opted out of publication is valid without a record", func(t *testing.T) {
		body := `{"results":[{"siren":"519341283","nom_complet":"[NON-DIFFUSIBLE]","nom_raison_sociale":"[NON-DIFFUSIBLE]","etat_administratif":"A","complements":{"est_entrepreneur_individuel":true},
"siege":{"numero_voie":"[NON-DIFFUSIBLE]","type_voie":"[NON-DIFFUSIBLE]","libelle_voie":"[NON-DIFFUSIBLE]","code_postal":"[NON-DIFFUSIBLE]","libelle_commune":"NIMES"}}]}`
		v, _, _ := setup(t, ok(body))
		ans, err := v.Verify(ctx, request("519341283"))
		require.NoError(t, err)
		assert.Equal(t, tin.StatusValid, ans.Status)
		assert.Nil(t, ans.Record)
	})

	t.Run("a CEDEX replaces the postcode and the locality", func(t *testing.T) {
		body := `{"results":[{"siren":"552032534","nom_raison_sociale":"DANONE","etat_administratif":"A",
"siege":{"numero_voie":"1","indice_repetition":"B","type_voie":"AVENUE","libelle_voie":"DE PARIS","complement_adresse":"BATIMENT A","distribution_speciale":"BP 12","code_postal":"75009","libelle_commune":"PARIS","cedex":"75441","libelle_cedex":"PARIS CEDEX 09"}}]}`
		v, _, _ := setup(t, ok(body))
		ans, err := v.Verify(ctx, request(danone))
		require.NoError(t, err)
		assert.Equal(t, &org.Address{Number: "1B", Street: "AVENUE DE PARIS", StreetExtra: "BATIMENT A", PostOfficeBox: "BP 12", Code: "75441", Locality: "PARIS CEDEX 09", Country: "FR"}, ans.Record.Address)
	})

	t.Run("a CEDEX without a postcode is an address", func(t *testing.T) {
		body := `{"results":[{"siren":"552032534","nom_raison_sociale":"DANONE","etat_administratif":"A",
"siege":{"numero_voie":"1","type_voie":"AVENUE","libelle_voie":"DE PARIS","code_postal":"","libelle_commune":"PARIS","cedex":"75441","libelle_cedex":"PARIS CEDEX 09"}}]}`
		v, _, _ := setup(t, ok(body))
		ans, err := v.Verify(ctx, request(danone))
		require.NoError(t, err)
		require.NotNil(t, ans.Record.Address)
		assert.Equal(t, cbc.Code("75441"), ans.Record.Address.Code)
		assert.Equal(t, "PARIS CEDEX 09", ans.Record.Address.Locality)
	})

	t.Run("a head office abroad gives no address", func(t *testing.T) {
		body := `{"results":[{"siren":"552032534","nom_raison_sociale":"DANONE","etat_administratif":"A","siege":{"libelle_voie":"MAIN STREET","code_postal":"","code_pays_etranger":"99132"}}]}`
		v, _, _ := setup(t, ok(body))
		ans, err := v.Verify(ctx, request(danone))
		require.NoError(t, err)
		assert.Equal(t, &tin.Record{Name: "DANONE"}, ans.Record)
	})

	t.Run("an answer without a results array is a network error", func(t *testing.T) {
		for _, body := range []string{`{}`, `{"results":null}`, `not json`} {
			v, _, _ := setup(t, ok(body))
			_, err := v.Verify(ctx, request(danone))
			assert.ErrorIs(t, err, tin.ErrNetwork, body)
		}
	})

	t.Run("a rate limit waits as the service says", func(t *testing.T) {
		v, _, _ := setup(t, reply{status: http.StatusTooManyRequests, body: `{}`, header: http.Header{"Retry-After": {"7"}}})
		_, err := v.Verify(ctx, request(danone))
		var rl *tin.RateLimitedError
		require.ErrorAs(t, err, &rl)
		assert.Equal(t, 7*time.Second, rl.RetryAfter)
		assert.Equal(t, "company search", rl.Reason)
	})

	t.Run("a rate limit without a delay waits a minute", func(t *testing.T) {
		v, _, _ := setup(t, reply{status: http.StatusTooManyRequests, body: `{}`})
		_, err := v.Verify(ctx, request(danone))
		var rl *tin.RateLimitedError
		require.ErrorAs(t, err, &rl)
		assert.Equal(t, time.Minute, rl.RetryAfter)
	})

	t.Run("a failing service is a server error", func(t *testing.T) {
		v, _, _ := setup(t, reply{status: http.StatusServiceUnavailable, body: "down"})
		_, err := v.Verify(ctx, request(danone))
		require.ErrorIs(t, err, tin.ErrServer)
		var te *tin.Error
		require.ErrorAs(t, err, &te)
		assert.Equal(t, "503", te.Code())
	})

	t.Run("a malformed SIREN is an input error, without a request", func(t *testing.T) {
		v, calls, _ := setup(t, ok(danoneCompany))
		for _, code := range []cbc.Code{"", "55203253", "5520325340", "55203253A", `552032534" OR "1"="1`} {
			_, err := v.Verify(ctx, request(code))
			assert.ErrorIs(t, err, tin.ErrInput, code)
		}
		assert.Equal(t, int32(0), calls.Load())
	})
}

func TestVerifierRouting(t *testing.T) {
	v := New()
	assert.Equal(t, Source, v.Source())
	assert.Equal(t, []cbc.Code{"SIREN"}, v.IdentityTypes())
	assert.True(t, v.Supports(tin.Identifier{Type: "SIREN", Code: danone}))
	assert.True(t, v.Supports(tin.Identifier{Type: "SIREN", Country: "FR", Code: danone}))
	assert.False(t, v.Supports(tin.Identifier{Type: "SIREN", Country: "BE", Code: danone}))
	assert.False(t, v.Supports(tin.Identifier{Type: "SIRET", Code: "55203253400646"}))
	assert.False(t, v.Supports(tin.Identifier{Country: "FR", Code: "27552032534"}), "a tax ID is not a SIREN")
}

func TestNewInvalidBaseURL(t *testing.T) {
	for _, u := range []string{"://bad", "ftp://example.com", "https://"} {
		_, err := New(WithURL(u)).Verify(context.Background(), request(danone))
		assert.ErrorIs(t, err, tin.ErrServer, u)
	}
}

// TestClient checks the client sends a party's SIREN identity to Sirene and
// writes the record back.
func TestClient(t *testing.T) {
	v, _, _ := setup(t, ok(danoneCompany))
	party := &org.Party{Name: "Danone SA", Identities: []*org.Identity{{Type: "SIREN", Code: "552 032 534"}}}
	report, err := tin.New(v).Verify(context.Background(), party)
	require.NoError(t, err)
	require.Len(t, report.Checks, 1)
	c := report.Checks[0]
	assert.Equal(t, "/identities/0", c.Path)
	assert.Equal(t, tin.StatusValid, c.Status)
	assert.Equal(t, Source, c.Source)
	require.Len(t, c.Mismatches, 1)
	assert.Equal(t, tin.MismatchName, c.Mismatches[0].Field)

	patch, err := tin.Patch(party, report)
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"DANONE","addresses":[{"num":"59","street":"RUE LA FAYETTE","locality":"PARIS","code":"75009","country":"FR"}]}`, string(patch))
}
