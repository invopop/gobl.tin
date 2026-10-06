package aeat

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl.tin/vies"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// response builds a VNifV2 response body for one taxpayer, in the shape of
// the examples in the AEAT technical manual.
func response(nif, name, result string) string {
	return `<env:Envelope xmlns:env="http://schemas.xmlsoap.org/soap/envelope/">
	<env:Body>
		<VNifV2Sal:VNifV2Sal xmlns:VNifV2Sal="http://www2.agenciatributaria.gob.es/static_files/common/internet/dep/aplicaciones/es/aeat/burt/jdit/ws/VNifV2Sal.xsd">
			<VNifV2Sal:Contribuyente>
				<VNifV2Sal:Nif>` + nif + `</VNifV2Sal:Nif>
				<VNifV2Sal:Nombre>` + name + `</VNifV2Sal:Nombre>
				<VNifV2Sal:Resultado>` + result + `</VNifV2Sal:Resultado>
			</VNifV2Sal:Contribuyente>
		</VNifV2Sal:VNifV2Sal>
	</env:Body>
</env:Envelope>`
}

// faultBody builds a SOAP fault body.
func faultBody(code, msg string) string {
	return `<env:Envelope xmlns:env="http://schemas.xmlsoap.org/soap/envelope/">
	<env:Body>
		<env:Fault>
			<faultcode>` + code + `</faultcode>
			<faultstring>` + msg + `</faultstring>
		</env:Fault>
	</env:Body>
</env:Envelope>`
}

// testCert returns a self-signed client certificate.
func testCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "gobl.tin test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// request is what the test server received.
type request struct {
	path        string
	contentType string
	body        string
}

// serve starts a test server that always answers with the given status and
// body, and returns a verifier pointed at it and the last request received.
func serve(t *testing.T, status int, body string) (*Verifier, *request) {
	t.Helper()
	got := new(request)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		*got = request{path: r.URL.Path, contentType: r.Header.Get("Content-Type"), body: string(data)}
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(testCert(t), WithBaseURL(srv.URL)), got
}

var (
	entity = tin.Identifier{Path: tin.PathTaxID, Country: "ES", Code: "B85905495"}
	person = tin.Identifier{Path: tin.PathTaxID, Country: "ES", Code: "99999999R"}
)

func TestNew(t *testing.T) {
	cert := testCert(t)

	t.Run("default base URL and timeout", func(t *testing.T) {
		v := New(cert)
		require.NoError(t, v.initErr)
		assert.Equal(t, DefaultBaseURL, v.baseURL)
		assert.Equal(t, DefaultTimeout, v.conn.GetClient().Timeout)
	})

	t.Run("WithSeal uses the seal host", func(t *testing.T) {
		assert.Equal(t, SealBaseURL, New(cert, WithSeal()).baseURL)
	})

	t.Run("WithBaseURL takes precedence over WithSeal", func(t *testing.T) {
		assert.Equal(t, "http://localhost:1", New(cert, WithSeal(), WithBaseURL("http://localhost:1")).baseURL)
	})

	t.Run("WithTimeout overrides the default", func(t *testing.T) {
		assert.Equal(t, 3*time.Second, New(cert, WithTimeout(3*time.Second)).conn.GetClient().Timeout)
	})

	t.Run("the transport presents the certificate", func(t *testing.T) {
		tr := transport(cert)
		require.NotNil(t, tr.TLSClientConfig)
		require.Len(t, tr.TLSClientConfig.Certificates, 1)
		assert.Equal(t, cert.Certificate, tr.TLSClientConfig.Certificates[0].Certificate)
		assert.Equal(t, tls.RenegotiateOnceAsClient, tr.TLSClientConfig.Renegotiation)
		assert.Nil(t, tr.TLSClientConfig.RootCAs, "server certificates are checked against the system roots")
		if dc := http.DefaultTransport.(*http.Transport).TLSClientConfig; dc != nil {
			assert.Empty(t, dc.Certificates, "the default transport is left as it was")
		}
	})

	t.Run("a missing certificate fails on the first call", func(t *testing.T) {
		v := New(tls.Certificate{})
		_, err := v.Verify(context.Background(), tin.Request{Identifier: entity})
		assert.ErrorIs(t, err, tin.ErrInput)
		assert.Contains(t, err.Error(), "no client certificate")
	})

	t.Run("an invalid base URL fails on the first call", func(t *testing.T) {
		for _, u := range []string{"://bad", "ftp://example.com", "https://"} {
			_, err := New(cert, WithBaseURL(u)).Verify(context.Background(), tin.Request{Identifier: entity})
			assert.ErrorIs(t, err, tin.ErrInput, u)
		}
	})

	t.Run("implements tin.Verifier", func(*testing.T) {
		var _ tin.Verifier = New(cert)
	})
}

func TestVerifier(t *testing.T) {
	t.Run("source", func(t *testing.T) {
		assert.Equal(t, Source, New(testCert(t)).Source())
	})

	t.Run("supports Spanish tax identities only", func(t *testing.T) {
		v := New(testCert(t))
		assert.True(t, v.Supports(entity))
		assert.True(t, v.Supports(person))
		assert.False(t, v.Supports(tin.Identifier{Country: "PT", Code: "545259045"}))
		assert.False(t, v.Supports(tin.Identifier{Code: "B85905495"}))
	})

	t.Run("sends the NIF and the party's name", func(t *testing.T) {
		v, got := serve(t, http.StatusOK, response("99999999R", "ESPAÑOL ESPAÑOL JUAN", "Identificado"))
		party := &org.Party{Name: " ESPAÑOL ESPAÑOL JUAN "}
		_, err := v.Verify(context.Background(), tin.Request{Identifier: person, Party: party})
		require.NoError(t, err)
		assert.Equal(t, vnifPath, got.path)
		assert.Equal(t, "application/xml", got.contentType)
		assert.Contains(t, got.body, `<vnif:VNifV2Ent><vnif:Contribuyente><vnif:Nif>99999999R</vnif:Nif><vnif:Nombre>ESPAÑOL ESPAÑOL JUAN</vnif:Nombre></vnif:Contribuyente></vnif:VNifV2Ent>`)
		assert.Contains(t, got.body, `xmlns:vnif="`+vnifNS+`"`)
	})

	t.Run("escapes the name", func(t *testing.T) {
		v, got := serve(t, http.StatusOK, response("B85905495", "A &amp; B SL", "Identificado"))
		_, err := v.Verify(context.Background(), tin.Request{Identifier: entity, Party: &org.Party{Name: "A & B <SL>"}})
		require.NoError(t, err)
		assert.Contains(t, got.body, `<vnif:Nombre>A &amp; B &lt;SL&gt;</vnif:Nombre>`)
	})

	t.Run("identifies an entity without a name", func(t *testing.T) {
		v, got := serve(t, http.StatusOK, response("B85905495", "INVOPOP SL", "Identificado"))
		ans, err := v.Verify(context.Background(), tin.Request{Identifier: entity})
		require.NoError(t, err)
		assert.Contains(t, got.body, `<vnif:Nombre></vnif:Nombre>`)
		assert.Equal(t, tin.StatusValid, ans.Status)
	})

	t.Run("a natural person without a name is an input error", func(t *testing.T) {
		v, got := serve(t, http.StatusOK, response("99999999R", "", "No identificado"))
		for name, party := range map[string]*org.Party{"no party": nil, "empty name": {Name: "  "}} {
			_, err := v.Verify(context.Background(), tin.Request{Identifier: person, Party: party})
			assert.ErrorIs(t, err, tin.ErrInput, name)
		}
		for _, code := range []cbc.Code{"X1234567L", "K1234567A"} {
			_, err := v.Verify(context.Background(), tin.Request{Identifier: tin.Identifier{Country: "ES", Code: code}})
			assert.ErrorIs(t, err, tin.ErrInput, code.String())
		}
		assert.Empty(t, got.body, "no request is sent")
	})

	t.Run("an empty code is an input error", func(t *testing.T) {
		v, _ := serve(t, http.StatusOK, response("", "", "Identificado"))
		_, err := v.Verify(context.Background(), tin.Request{Identifier: tin.Identifier{Country: "ES"}})
		assert.ErrorIs(t, err, tin.ErrInput)
	})

	tests := []struct {
		name   string
		result string
		status tin.Status
		record *tin.Record
	}{
		{"identified", "Identificado", tin.StatusValid, &tin.Record{Name: "INVOPOP SL"}},
		{"identified in upper case", "IDENTIFICADO", tin.StatusValid, &tin.Record{Name: "INVOPOP SL"}},
		{"identified in state baja", "Identificado-Baja", tin.StatusValid, &tin.Record{Name: "INVOPOP SL", Status: tin.RecordStatusInactive}},
		{"identified with a revoked NIF", "Identificado-Revocado", tin.StatusValid, &tin.Record{Name: "INVOPOP SL", Status: tin.RecordStatusDissolved}},
		{"not identified echoes the name, which is no record", "No identificado", tin.StatusInvalid, nil},
		{"not identified by a similar name holds the registered name", "No identificado -similar", tin.StatusInvalid, &tin.Record{Name: "INVOPOP SL"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, _ := serve(t, http.StatusOK, response("B85905495", " INVOPOP SL ", tt.result))
			before := time.Now().Add(-time.Second)
			ans, err := v.Verify(context.Background(), tin.Request{Identifier: entity, Party: &org.Party{Name: "Invopop"}})
			require.NoError(t, err)
			assert.Equal(t, tt.status, ans.Status)
			assert.Equal(t, tt.record, ans.Record)
			assert.True(t, ans.CheckedAt.After(before))
			require.NotNil(t, ans.TaxID)
			assert.Equal(t, "ES", ans.TaxID.Country.String())
			assert.Equal(t, "B85905495", ans.TaxID.Code.String())
		})
	}

	t.Run("an empty name with a status is a record", func(t *testing.T) {
		v, _ := serve(t, http.StatusOK, response("B85905495", "", "Identificado-Baja"))
		ans, err := v.Verify(context.Background(), tin.Request{Identifier: entity})
		require.NoError(t, err)
		assert.Equal(t, &tin.Record{Status: tin.RecordStatusInactive}, ans.Record)
	})

	t.Run("an empty name without a status is no record", func(t *testing.T) {
		v, _ := serve(t, http.StatusOK, response("B85905495", "", "Identificado"))
		ans, err := v.Verify(context.Background(), tin.Request{Identifier: entity})
		require.NoError(t, err)
		assert.Nil(t, ans.Record)
	})

	t.Run("an empty NIF echo falls back to the request", func(t *testing.T) {
		v, _ := serve(t, http.StatusOK, response("", "INVOPOP SL", "Identificado"))
		ans, err := v.Verify(context.Background(), tin.Request{Identifier: entity})
		require.NoError(t, err)
		assert.Equal(t, "B85905495", ans.TaxID.Code.String())
	})

	t.Run("an unknown result is a server error", func(t *testing.T) {
		v, _ := serve(t, http.StatusOK, response("B85905495", "INVOPOP SL", "No procesado"))
		_, err := v.Verify(context.Background(), tin.Request{Identifier: entity})
		assert.ErrorIs(t, err, tin.ErrServer)
		assert.Contains(t, err.Error(), "No procesado")
		var e *tin.Error
		require.True(t, errors.As(err, &e))
		assert.Equal(t, "200", e.Code())
	})
}

func TestVerifyTransport(t *testing.T) {
	t.Run("a client fault is an input error", func(t *testing.T) {
		v, _ := serve(t, http.StatusInternalServerError, faultBody("env:Client", "Codigo[-2].Sólo se permiten caracteres UTF-8"))
		ans, err := v.Verify(context.Background(), tin.Request{Identifier: entity})
		assert.Nil(t, ans)
		assert.ErrorIs(t, err, tin.ErrInput)
		assert.Contains(t, err.Error(), "Codigo[-2]")
		var e *tin.Error
		require.True(t, errors.As(err, &e))
		assert.Equal(t, "500", e.Code())
	})

	t.Run("a fault with status 200 is still a fault", func(t *testing.T) {
		v, _ := serve(t, http.StatusOK, faultBody("env:Client", "Codigo[103].No se ha encontrado la etiqueta de inicio"))
		_, err := v.Verify(context.Background(), tin.Request{Identifier: entity})
		assert.ErrorIs(t, err, tin.ErrInput)
	})

	t.Run("a server fault is a server error", func(t *testing.T) {
		v, _ := serve(t, http.StatusInternalServerError, faultBody("env:Server", "Error interno"))
		_, err := v.Verify(context.Background(), tin.Request{Identifier: entity})
		assert.ErrorIs(t, err, tin.ErrServer)
		assert.Contains(t, err.Error(), "Error interno")
	})

	t.Run("a rejected certificate is a server error", func(t *testing.T) {
		v, _ := serve(t, http.StatusForbidden, `<html>403 Forbidden</html>`)
		_, err := v.Verify(context.Background(), tin.Request{Identifier: entity})
		assert.ErrorIs(t, err, tin.ErrServer)
		var e *tin.Error
		require.True(t, errors.As(err, &e))
		assert.Equal(t, "403", e.Code())
	})

	t.Run("a bad request is an input error", func(t *testing.T) {
		v, _ := serve(t, http.StatusBadRequest, `bad request`)
		_, err := v.Verify(context.Background(), tin.Request{Identifier: entity})
		assert.ErrorIs(t, err, tin.ErrInput)
	})

	t.Run("rate limited", func(t *testing.T) {
		v, _ := serve(t, http.StatusTooManyRequests, ``)
		_, err := v.Verify(context.Background(), tin.Request{Identifier: entity})
		var rl *tin.RateLimitedError
		require.True(t, errors.As(err, &rl))
		assert.Equal(t, defaultRetryAfter, rl.RetryAfter)
	})

	t.Run("a rate limit wins over a fault in the body", func(t *testing.T) {
		v, _ := serve(t, http.StatusTooManyRequests, faultBody("env:Client", "Codigo[103]"))
		_, err := v.Verify(context.Background(), tin.Request{Identifier: entity})
		var rl *tin.RateLimitedError
		require.True(t, errors.As(err, &rl))
	})

	t.Run("an unreadable body is a network error", func(t *testing.T) {
		v, _ := serve(t, http.StatusOK, `<html>maintenance`)
		_, err := v.Verify(context.Background(), tin.Request{Identifier: entity})
		assert.ErrorIs(t, err, tin.ErrNetwork)
	})

	t.Run("a body without a taxpayer is a network error", func(t *testing.T) {
		v, _ := serve(t, http.StatusOK, `<env:Envelope xmlns:env="http://schemas.xmlsoap.org/soap/envelope/"><env:Body/></env:Envelope>`)
		_, err := v.Verify(context.Background(), tin.Request{Identifier: entity})
		assert.ErrorIs(t, err, tin.ErrNetwork)
		assert.Contains(t, err.Error(), "no taxpayer")
	})

	t.Run("a dial failure is a network error", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		srv.Close()
		_, err := New(testCert(t), WithBaseURL(srv.URL)).Verify(context.Background(), tin.Request{Identifier: entity})
		assert.ErrorIs(t, err, tin.ErrNetwork)
	})

	t.Run("messages are clamped to valid text", func(t *testing.T) {
		v, _ := serve(t, http.StatusInternalServerError, faultBody("env:Server", strings.Repeat("ñ", 300)))
		_, err := v.Verify(context.Background(), tin.Request{Identifier: entity})
		var e *tin.Error
		require.True(t, errors.As(err, &e))
		assert.LessOrEqual(t, len(e.Message()), maxMessage)
		assert.True(t, utf8.ValidString(e.Message()))
	})
}

// TestClient checks the AEAT verifier behind a tin.Client: mismatches and
// the patch.
func TestClient(t *testing.T) {
	t.Run("the registered name is a mismatch and is patched", func(t *testing.T) {
		v, _ := serve(t, http.StatusOK, response("B85905495", "INVOPOP SL", "Identificado"))
		party := &org.Party{Name: "Invopop", TaxID: &tax.Identity{Country: "ES", Code: "B85905495"}}
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
		assert.JSONEq(t, `{"name":"INVOPOP SL"}`, string(patch))
	})

	t.Run("a similar name is shown, but never patched", func(t *testing.T) {
		v, _ := serve(t, http.StatusOK, response("99999999R", "ESPAÑOL ESPAÑOL JUAN", "No identificado -similar"))
		party := &org.Party{Name: "ESPANOL ESPANOL JUAN", TaxID: &tax.Identity{Country: "ES", Code: "99999999R"}}
		report, err := tin.New(v).Verify(context.Background(), party)
		require.NoError(t, err)
		check := report.Checks[0]
		assert.Equal(t, tin.StatusInvalid, check.Status)
		require.Len(t, check.Mismatches, 1)
		assert.Equal(t, "ESPAÑOL ESPAÑOL JUAN", check.Mismatches[0].Register)
		patch, err := tin.Patch(party, report)
		require.NoError(t, err)
		assert.JSONEq(t, `{}`, string(patch))
	})

	t.Run("a new NIF for the entity is a tax ID mismatch", func(t *testing.T) {
		v, _ := serve(t, http.StatusOK, response("B85905495", "INVOPOP SL", "Identificado"))
		party := &org.Party{Name: "INVOPOP SL", TaxID: &tax.Identity{Country: "ES", Code: "A85905495"}}
		report, err := tin.New(v).Verify(context.Background(), party)
		require.NoError(t, err)
		check := report.Checks[0]
		require.Len(t, check.Mismatches, 1)
		assert.Equal(t, tin.MismatchTaxID, check.Mismatches[0].Field)
		assert.Equal(t, "/tax_id/code", check.Mismatches[0].Path)
		assert.Equal(t, "B85905495", check.Mismatches[0].Register)
	})

	t.Run("AEAT answers before VIES when it comes first", func(t *testing.T) {
		v, _ := serve(t, http.StatusOK, response("B85905495", "INVOPOP SL", "Identificado"))
		c := tin.New(v, vies.New(vies.WithBaseURL("http://127.0.0.1:1")))
		check, err := c.VerifyTaxID(context.Background(), &tax.Identity{Country: "ES", Code: "ESB85905495"})
		require.NoError(t, err)
		assert.Equal(t, tin.StatusValid, check.Status)
	})
}

func TestNormalizeResult(t *testing.T) {
	for in, want := range map[string]string{
		"Identificado":             resultIdentified,
		" identificado ":           resultIdentified,
		"Identificado-Baja":        resultIdentifiedDeregistered,
		"IDENTIFICADO-REVOCADO":    resultIdentifiedRevoked,
		"No identificado":          resultNotIdentified,
		"No identificado -similar": resultNotIdentifiedSimilar,
		"No identificado-Similar":  resultNotIdentifiedSimilar,
	} {
		assert.Equal(t, want, normalizeResult(in), in)
	}
}
