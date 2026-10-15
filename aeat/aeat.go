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

// DefaultBaseURL is the AEAT host for personal and representative certificates.
const DefaultBaseURL = "https://www1.agenciatributaria.gob.es"

// SealBaseURL is the AEAT host for seal certificates (certificados de sello).
const SealBaseURL = "https://www10.agenciatributaria.gob.es"

// DefaultTimeout bounds a single API call.
const DefaultTimeout = 15 * time.Second

const vnifPath = "/wlpl/BURT-JDIT/ws/VNifV2SOAP"

// defaultRetryAfter is used when AEAT rate limits.
const defaultRetryAfter = time.Minute

// Namespaces of the VNifV2 request.
const (
	soapNS = "http://schemas.xmlsoap.org/soap/envelope/"
	vnifNS = "http://www2.agenciatributaria.gob.es/static_files/common/internet/dep/aplicaciones/es/aeat/burt/jdit/ws/VNifV2Ent.xsd"
)

// All possible results of the VNifV2 service, upper-cased.
const (
	// NIF exists (and name matches for natural person).
	resultIdentified = "IDENTIFICADO"

	// NIF exists in state 'baja'.
	resultIdentifiedDeregistered = "IDENTIFICADO-BAJA"

	// NIF exists in state 'baja' because it was revoked.
	resultIdentifiedRevoked = "IDENTIFICADO-REVOCADO"

	// NIF not found, or not found with the given name.
	resultNotIdentified = "NOIDENTIFICADO"

	// NIF found with similar name.
	resultNotIdentifiedSimilar = "NOIDENTIFICADO-SIMILAR"
)

const reasonPersonNotIdentified = "AEAT does not identify the DNI or NIE with the name given"

// Placeholder NIF that AEAT uses for operations without an identified party.
const placeholderNIF cbc.Code = "B99999997"

// Verifier is the AEAT verifier. It implements tin.Verifier.
type Verifier struct {
	baseURL string
	seal    bool
	timeout time.Duration
	conn    *resty.Client

	// Returned by Verify when New got an invalid configuration.
	configErr error
}

// Option configures the Verifier.
type Option func(*Verifier)

// WithBaseURL points the API at a different endpoint, for tests. It takes
// precedence over WithSeal.
func WithBaseURL(url string) Option {
	return func(v *Verifier) {
		v.baseURL = url
	}
}

// WithSeal sends requests to SealBaseURL, required by AEAT for seal certificates.
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

// New creates an AEAT verifier with mutual TLS (certificate on every request).
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
		v.configErr = tin.ErrServer.WithMessage("no client certificate provided")
	}
	if u, err := url.Parse(v.baseURL); err != nil {
		v.configErr = tin.ErrServer.WithMsgf("invalid base URL %q", v.baseURL).WithCause(err)
	} else if u.Scheme != "http" && u.Scheme != "https" {
		v.configErr = tin.ErrServer.WithMsgf("invalid base URL %q: scheme must be http or https", v.baseURL)
	} else if u.Hostname() == "" {
		v.configErr = tin.ErrServer.WithMsgf("invalid base URL %q: missing host", v.baseURL)
	}
	v.conn = resty.NewWithClient(&http.Client{Transport: transport(cert)}).
		SetBaseURL(v.baseURL).
		SetTimeout(v.timeout)
	return v
}

// HTTP transport that presents the certificate. Allows TLS renegotiation
// (required by AEAT).
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

type requestEnvelope struct {
	XMLName xml.Name `xml:"soapenv:Envelope"`
	SoapNS  string   `xml:"xmlns:soapenv,attr"`
	VnifNS  string   `xml:"xmlns:vnif,attr"`
	Header  string   `xml:"soapenv:Header"`
	Body    struct {
		Taxpayer taxpayerRequest `xml:"vnif:VNifV2Ent>vnif:Contribuyente"`
	} `xml:"soapenv:Body"`
}

type taxpayerRequest struct {
	NIF  string `xml:"vnif:Nif"`
	Name string `xml:"vnif:Nombre"`
}

type responseEnvelope struct {
	Body struct {
		Fault  *fault `xml:"Fault"`
		Output *struct {
			Taxpayers []taxpayerResponse `xml:"Contribuyente"`
		} `xml:"VNifV2Sal"`
	} `xml:"Body"`
}

type taxpayerResponse struct {
	NIF    string `xml:"Nif"`
	Name   string `xml:"Nombre"`
	Result string `xml:"Resultado"`
}

// SOAP fault: AEAT's answer to a malformed request.
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

// PartyKey returns the normalized name of a natural person, or "" for an
// entity.
func (v *Verifier) PartyKey(req tin.Request) string {
	if req.Party == nil || !naturalPerson(req.Identifier.Code) {
		return ""
	}
	return strings.Join(strings.Fields(strings.ToUpper(req.Party.Name)), " ")
}

// Verify identifies the NIF in the AEAT census. A natural person needs a name.
func (v *Verifier) Verify(ctx context.Context, req tin.Request) (*tin.Answer, error) {
	if v.configErr != nil {
		return nil, v.configErr
	}
	id := req.Identifier
	if id.Code == "" {
		return nil, tin.ErrInput.WithMessage("no tax ID code provided")
	}
	name := ""
	if req.Party != nil {
		name = strings.TrimSpace(req.Party.Name)
	}
	if id.Code == placeholderNIF {
		return &tin.Answer{
			Status:    tin.StatusInvalid,
			TaxID:     &tax.Identity{Country: l10n.ES.Tax(), Code: id.Code},
			CheckedAt: time.Now().UTC(),
		}, nil
	}
	if name == "" && naturalPerson(id.Code) {
		return nil, tin.ErrInput.WithMessage("AEAT needs the name of a natural person to check a DNI or NIE")
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
	result := normalizeResult(out.Result)
	switch result {
	case resultIdentified, resultIdentifiedDeregistered, resultIdentifiedRevoked:
		ans.Status = tin.StatusValid
		// AEAT accepts small differences in a natural person's name.
		rec.NameConfirmed = naturalPerson(id.Code)
		switch result {
		case resultIdentifiedDeregistered:
			rec.Status = tin.RecordStatusInactive
		case resultIdentifiedRevoked:
			rec.Status = tin.RecordStatusRevoked
		}
	case resultNotIdentifiedSimilar:
		ans.Status = tin.StatusInvalid
	case resultNotIdentified:
		ans.Status = tin.StatusInvalid
		rec = nil
	default:
		return nil, tin.ErrServer.WithCode(status).WithMsgf("unknown result %q", clamp(out.Result))
	}
	if ans.Status == tin.StatusInvalid && naturalPerson(id.Code) {
		ans.Reason = reasonPersonNotIdentified
	}
	if rec != nil && (rec.Name != "" || rec.Status != "") {
		ans.Record = rec
	}
	return ans, nil
}

// Natural person: a DNI, a NIE, or a K, L or M code.
func naturalPerson(code cbc.Code) bool {
	tID := &tax.Identity{Country: l10n.ES.Tax(), Code: code}
	return es.TaxIdentityKey(tID).In(es.TaxIdentityNational, es.TaxIdentityForeigner, es.TaxIdentityOther)
}

// Posts one taxpayer to the VNifV2 service and returns the outcome.
func (v *Verifier) identify(ctx context.Context, code cbc.Code, name string) (*taxpayerResponse, string, error) {
	env := &requestEnvelope{SoapNS: soapNS, VnifNS: vnifNS}
	env.Body.Taxpayer = taxpayerRequest{NIF: code.String(), Name: name}
	body, err := xml.Marshal(env)
	if err != nil {
		return nil, "", tin.ErrServer.WithMessage("encoding request").WithCause(err)
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
		return nil, status, statusError(resp)
	}
	out := new(responseEnvelope)
	decodeErr := xml.Unmarshal(resp.Body(), out)
	if decodeErr == nil && out.Body.Fault != nil {
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

// Maps a SOAP fault onto the tin error taxonomy.
func faultError(status string, f *fault) error {
	msg := clamp(strings.TrimSpace(f.Message))
	code := strings.TrimSpace(f.Code)
	if code == "Client" || strings.HasSuffix(code, ":Client") {
		return tin.ErrInput.WithCode(status).WithMessage(msg)
	}
	return tin.ErrServer.WithCode(status).WithMessage(msg)
}

// Maps non-2xx responses without SOAP fault to the tin error taxonomy.
func statusError(resp *resty.Response) error {
	code := resp.StatusCode()
	status := strconv.Itoa(code)
	switch code {
	case http.StatusBadRequest:
		return tin.ErrInput.WithCode(status).WithMessage(clamp(string(resp.Body())))
	case http.StatusTooManyRequests:
		return &tin.RateLimitedError{RetryAfter: defaultRetryAfter}
	default:
		// Everything else is the register failing to answer.
		return tin.ErrServer.WithCode(status).WithMessage(clamp(string(resp.Body())))
	}
}

// Tax identity as AEAT returns it, or as sent.
func confirmedTaxID(code cbc.Code, out *taxpayerResponse) *tax.Identity {
	tid := &tax.Identity{Country: l10n.ES.Tax(), Code: cbc.Code(strings.TrimSpace(out.NIF))}
	if tid.Code == "" {
		tid.Code = code
	}
	return tid
}

func normalizeResult(s string) string {
	return strings.Join(strings.Fields(strings.ToUpper(s)), "")
}

// Maximum length of response text in an error.
const maxMessage = 200

func clamp(s string) string {
	if len(s) > maxMessage {
		s = s[:maxMessage]
	}
	return strings.ToValidUTF8(s, "")
}
