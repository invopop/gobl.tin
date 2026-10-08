// Package aeat implements the verifier for the census of AEAT, the Spanish
// tax agency, through its VNifV2 identification service.
package aeat

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/regimes/es"
	"github.com/invopop/gobl/tax"
)

// Source names AEAT as the register in checks.
const Source cbc.Key = "aeat"

// DefaultBaseURL is the AEAT host for personal and representative
// certificates. The census is read only, and the AEAT test environment holds
// no real taxpayers, so checks always run against production.
const DefaultBaseURL = "https://www1.agenciatributaria.gob.es"

// SealBaseURL is the AEAT host for seal certificates (certificados de sello).
const SealBaseURL = "https://www10.agenciatributaria.gob.es"

// DefaultTimeout bounds a single API call.
const DefaultTimeout = 15 * time.Second

const vnifPath = "/wlpl/BURT-JDIT/ws/VNifV2SOAP"

// defaultRetryAfter is used when AEAT rate limits. AEAT does not document
// its limits, so this is a polite guess.
const defaultRetryAfter = time.Minute

// Namespaces of the VNifV2 request. AEAT's own example uses http for the
// request schema, and gov-es sends it the same way.
const (
	soapNS = "http://schemas.xmlsoap.org/soap/envelope/"
	vnifNS = "http://www2.agenciatributaria.gob.es/static_files/common/internet/dep/aplicaciones/es/aeat/burt/jdit/ws/VNifV2Ent.xsd"
)

// Results of the VNifV2 service, upper cased and without spaces. AEAT
// writes them in mixed case, such as "No identificado -similar".
const (
	// resultIdentified means the NIF exists and, for a natural person, the
	// name matches.
	resultIdentified = "IDENTIFICADO"

	// resultIdentifiedDeregistered means an entity with the NIF exists in
	// state baja.
	resultIdentifiedDeregistered = "IDENTIFICADO-BAJA"

	// resultIdentifiedRevoked means an entity with the NIF exists in state
	// baja because AEAT revoked the NIF.
	resultIdentifiedRevoked = "IDENTIFICADO-REVOCADO"

	// resultNotIdentified means AEAT does not hold the NIF, or does not
	// hold it with the name given.
	resultNotIdentified = "NOIDENTIFICADO"

	// resultNotIdentifiedSimilar means a natural person with the NIF exists,
	// but the name given differs in small details. AEAT returns its name.
	resultNotIdentifiedSimilar = "NOIDENTIFICADO-SIMILAR"
)

// Verifier is the AEAT verifier. It implements tin.Verifier.
type Verifier struct {
	baseURL string
	seal    bool
	timeout time.Duration
	conn    *resty.Client

	// initErr records an invalid construction, such as a missing
	// certificate or an unparseable base URL. Verify returns it, so a
	// misconfiguration surfaces on the first call.
	initErr error
}

// Option configures the Verifier.
type Option func(*Verifier)

// WithBaseURL points the API at a different endpoint. It exists so that tests
// can run against a local server instead of the live AEAT service. It takes
// precedence over WithSeal.
func WithBaseURL(url string) Option {
	return func(v *Verifier) {
		v.baseURL = url
	}
}

// WithSeal sends requests to SealBaseURL, which AEAT requires for a seal
// certificate.
func WithSeal() Option {
	return func(v *Verifier) {
		v.seal = true
	}
}

// WithTimeout overrides the default bound on a single API call.
func WithTimeout(d time.Duration) Option {
	return func(v *Verifier) {
		v.timeout = d
	}
}

// New creates an AEAT verifier. AEAT requires mutual TLS, so the verifier
// presents the certificate on every request. The certificate must be one
// that the AEAT electronic office accepts: a personal, representative or
// seal certificate.
func New(cert tls.Certificate, opts ...Option) *Verifier {
	v := &Verifier{timeout: DefaultTimeout}
	for _, opt := range opts {
		opt(v)
	}
	if v.baseURL == "" {
		v.baseURL = DefaultBaseURL
		if v.seal {
			v.baseURL = SealBaseURL
		}
	}
	if len(cert.Certificate) == 0 || cert.PrivateKey == nil {
		v.initErr = tin.ErrInput.WithMessage("no client certificate provided")
	}
	if u, err := url.Parse(v.baseURL); err != nil {
		v.initErr = tin.ErrInput.WithMsgf("invalid base URL %q", v.baseURL).WithCause(err)
	} else if u.Scheme != "http" && u.Scheme != "https" {
		v.initErr = tin.ErrInput.WithMsgf("invalid base URL %q: scheme must be http or https", v.baseURL)
	} else if u.Hostname() == "" {
		v.initErr = tin.ErrInput.WithMsgf("invalid base URL %q: missing host", v.baseURL)
	}
	v.conn = resty.NewWithClient(&http.Client{Transport: transport(cert)}).
		SetBaseURL(v.baseURL).
		SetTimeout(v.timeout)
	return v
}

// transport builds an HTTP transport that presents the certificate. AEAT
// asks for the client certificate through a TLS renegotiation, so the
// transport allows one. Server certificates are checked against the system
// roots.
func transport(cert tls.Certificate) *http.Transport {
	tr, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		tr = tr.Clone()
	} else {
		tr = new(http.Transport)
	}
	tr.TLSClientConfig = &tls.Config{
		MinVersion:    tls.VersionTLS12,
		Certificates:  []tls.Certificate{cert},
		Renegotiation: tls.RenegotiateOnceAsClient,
	}
	return tr
}

// requestEnvelope is the SOAP request for one taxpayer.
type requestEnvelope struct {
	XMLName xml.Name `xml:"soapenv:Envelope"`
	SoapNS  string   `xml:"xmlns:soapenv,attr"`
	VnifNS  string   `xml:"xmlns:vnif,attr"`
	Header  string   `xml:"soapenv:Header"`
	Body    struct {
		Taxpayer taxpayerRequest `xml:"vnif:VNifV2Ent>vnif:Contribuyente"`
	} `xml:"soapenv:Body"`
}

// taxpayerRequest is the NIF and the name to identify.
type taxpayerRequest struct {
	NIF  string `xml:"vnif:Nif"`
	Name string `xml:"vnif:Nombre"`
}

// responseEnvelope is the SOAP response, matched on local names so that any
// namespace prefix works.
type responseEnvelope struct {
	Body struct {
		Fault  *fault `xml:"Fault"`
		Output *struct {
			Taxpayers []taxpayerResponse `xml:"Contribuyente"`
		} `xml:"VNifV2Sal"`
	} `xml:"Body"`
}

// taxpayerResponse is the outcome for one taxpayer.
type taxpayerResponse struct {
	NIF    string `xml:"Nif"`
	Name   string `xml:"Nombre"`
	Result string `xml:"Resultado"`
}

// fault is a SOAP fault. AEAT answers with one when the request is badly
// formed, with the code env:Client.
type fault struct {
	Code    string `xml:"faultcode"`
	Message string `xml:"faultstring"`
}

// Source names AEAT as the register.
func (v *Verifier) Source() cbc.Key {
	return Source
}

// Supports reports whether the identifier is a Spanish tax identity.
func (v *Verifier) Supports(id tin.Identifier) bool {
	return id.Country == l10n.ES.Tax()
}

// Verify identifies the request's NIF in the AEAT census. AEAT needs the
// name of a natural person, so the party's name is sent with the NIF. An
// entity is identified from the NIF alone. A NIF that AEAT does not
// identify is an Answer with StatusInvalid, not an error.
func (v *Verifier) Verify(ctx context.Context, req tin.Request) (*tin.Answer, error) {
	if v.initErr != nil {
		return nil, v.initErr
	}
	id := req.Identifier
	if id.Code == "" {
		return nil, tin.ErrInput.WithMessage("no tax ID code provided")
	}
	name := ""
	if req.Party != nil {
		name = strings.TrimSpace(req.Party.Name)
	}
	if name == "" && naturalPerson(id.Code) {
		return nil, tin.ErrInput.WithMessage("AEAT needs the name of a natural person")
	}

	out, status, err := v.identify(ctx, id.Code, name)
	if err != nil {
		return nil, err
	}
	ans := &tin.Answer{
		TaxID:     confirmedTaxID(id.Code, out),
		CheckedAt: time.Now().UTC(),
	}
	rec := &tin.Record{Name: strings.TrimSpace(out.Name)}
	switch normalizeResult(out.Result) {
	case resultIdentified:
		ans.Status = tin.StatusValid
		// AEAT compares the name of a natural person and accepts small
		// differences, so its answer confirms the name sent. It writes the
		// name its own way, with the surnames first.
		rec.NameConfirmed = naturalPerson(id.Code)
	case resultIdentifiedDeregistered:
		ans.Status = tin.StatusValid
		rec.Status = tin.RecordStatusInactive
	case resultIdentifiedRevoked:
		ans.Status = tin.StatusValid
		rec.Status = tin.RecordStatusDissolved
	case resultNotIdentifiedSimilar:
		// AEAT returns the name it holds, so the mismatch shows it.
		ans.Status = tin.StatusInvalid
	case resultNotIdentified:
		// AEAT echoes the name given, which is no record.
		ans.Status = tin.StatusInvalid
		rec = nil
	default:
		return nil, tin.ErrServer.WithCode(status).WithMsgf("unknown result %q", clamp(out.Result))
	}
	if rec != nil && (rec.Name != "" || rec.Status != "") {
		ans.Record = rec
	}
	return ans, nil
}

// naturalPerson reports whether the NIF belongs to a natural person: a DNI,
// a NIE, or a K, L or M code.
func naturalPerson(code cbc.Code) bool {
	switch es.TaxIdentityKey(&tax.Identity{Country: l10n.ES.Tax(), Code: code}) {
	case es.TaxIdentityNational, es.TaxIdentityForeigner, es.TaxIdentityOther:
		return true
	default:
		return false
	}
}

// identify posts one taxpayer to the VNifV2 service and returns the outcome
// with the HTTP status of the response.
func (v *Verifier) identify(ctx context.Context, code cbc.Code, name string) (*taxpayerResponse, string, error) {
	env := &requestEnvelope{SoapNS: soapNS, VnifNS: vnifNS}
	env.Body.Taxpayer = taxpayerRequest{NIF: code.String(), Name: name}
	body, err := xml.Marshal(env)
	if err != nil {
		return nil, "", tin.ErrInput.WithMessage("encoding request").WithCause(err)
	}

	resp, err := v.conn.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/xml").
		SetBody(body).
		Post(vnifPath)
	if err != nil {
		return nil, "", tin.ErrNetwork.WithCause(err)
	}

	status := strconv.Itoa(resp.StatusCode())
	if resp.StatusCode() == http.StatusTooManyRequests {
		// A rate limit wins over the body, so that callers back off.
		return nil, status, statusError(resp)
	}
	out := new(responseEnvelope)
	decodeErr := xml.Unmarshal(resp.Body(), out)
	if decodeErr == nil && out.Body.Fault != nil {
		// SOAP 1.1 sends a fault with status 500, and AEAT may also send
		// one with status 200.
		return nil, status, faultError(status, out.Body.Fault)
	}
	if !resp.IsSuccess() {
		return nil, status, statusError(resp)
	}
	if decodeErr != nil {
		return nil, status, tin.ErrNetwork.WithCode(status).WithMessage("decoding response").WithCause(decodeErr)
	}
	if out.Body.Output == nil || len(out.Body.Output.Taxpayers) == 0 {
		return nil, status, tin.ErrNetwork.WithCode(status).WithMessage("response carries no taxpayer")
	}
	return &out.Body.Output.Taxpayers[0], status, nil
}

// faultError maps a SOAP fault onto the tin error taxonomy: a client fault
// is a request that AEAT rejects as badly formed, and every other fault is
// the register failing to answer.
func faultError(status string, f *fault) error {
	msg := clamp(strings.TrimSpace(f.Message))
	code := strings.TrimSpace(f.Code)
	if code == "Client" || strings.HasSuffix(code, ":Client") {
		return tin.ErrInput.WithCode(status).WithMessage(msg)
	}
	return tin.ErrServer.WithCode(status).WithMessage(msg)
}

// statusError maps a non-2xx response without a SOAP fault onto the tin
// error taxonomy.
func statusError(resp *resty.Response) error {
	code := resp.StatusCode()
	status := strconv.Itoa(code)
	switch code {
	case http.StatusBadRequest:
		return tin.ErrInput.WithCode(status).WithMessage(clamp(string(resp.Body())))
	case http.StatusTooManyRequests:
		return &tin.RateLimitedError{RetryAfter: defaultRetryAfter}
	default:
		// Everything else, including a 403 for a certificate that AEAT
		// does not accept, is the register failing to answer.
		return tin.ErrServer.WithCode(status).WithMessage(clamp(string(resp.Body())))
	}
}

// confirmedTaxID builds the tax identity as AEAT confirmed it. AEAT returns
// the NIF padded to nine characters, and the current NIF of an entity whose
// first letter changed with its legal form.
func confirmedTaxID(code cbc.Code, out *taxpayerResponse) *tax.Identity {
	tid := &tax.Identity{Country: l10n.ES.Tax(), Code: cbc.Code(strings.TrimSpace(out.NIF))}
	if tid.Code == "" {
		tid.Code = code
	}
	return tid
}

// normalizeResult upper cases a result and removes its spaces, so that
// "No identificado -similar" reads as "NOIDENTIFICADO-SIMILAR".
func normalizeResult(s string) string {
	return strings.Join(strings.Fields(strings.ToUpper(s)), "")
}

// maxMessage bounds any text taken from an AEAT response into an error, so an
// untrusted body cannot make a failure arbitrarily large.
const maxMessage = 200

// clamp cuts s to maxMessage bytes and drops invalid UTF-8, so a message
// taken from an untrusted body is always valid text.
func clamp(s string) string {
	if len(s) > maxMessage {
		s = s[:maxMessage]
	}
	return strings.ToValidUTF8(s, "")
}
