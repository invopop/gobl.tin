// Package vies implements the verifier for VIES, the European Commission's
// VAT number register.
package vies

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/tax"
)

// Source names VIES as the register in checks.
const Source cbc.Key = "vies"

// DefaultBaseURL is the production VIES REST endpoint. VIES checks are read
// only and the Commission provides no test environment, so checks always run
// against production.
const DefaultBaseURL = "https://ec.europa.eu/taxation_customs/vies/rest-api"

// DefaultTimeout bounds a single API call.
const DefaultTimeout = 15 * time.Second

// userAgent identifies this library to the Commission's edge, which blocks
// clients that send default Go user agents.
const userAgent = "gobl.tin (+https://github.com/invopop/gobl.tin)"

const checkVatPath = "/check-vat-number"

// defaultRetryAfter is used when VIES rate limits without a Retry-After
// header. VIES does not document its limits, so this is a polite guess.
const defaultRetryAfter = time.Minute

// countryCodes lists the tax country codes VIES covers. VIES uses EL for
// Greece and XI for Northern Ireland.
var countryCodes = []l10n.Code{
	l10n.AT, l10n.BE, l10n.BG, l10n.CY, l10n.CZ, l10n.DE, l10n.DK, l10n.EE, l10n.EL, l10n.ES,
	l10n.FI, l10n.FR, l10n.HR, l10n.HU, l10n.IE, l10n.IT, l10n.LT, l10n.LU, l10n.LV, l10n.MT,
	l10n.NL, l10n.PL, l10n.PT, l10n.RO, l10n.SE, l10n.SI, l10n.SK, l10n.XI,
}

// Verifier is the VIES verifier. It implements tin.Verifier.
type Verifier struct {
	baseURL string
	timeout time.Duration
	http    *http.Client
	conn    *resty.Client

	// initErr records an invalid construction, such as an unparseable base
	// URL. Verify returns it, so a misconfiguration surfaces on the first call.
	initErr error
}

// Option configures the Verifier.
type Option func(*Verifier)

// WithBaseURL points the API at a different endpoint. It exists so that tests
// can run against a local server instead of the live VIES service.
func WithBaseURL(url string) Option {
	return func(a *Verifier) {
		a.baseURL = url
	}
}

// WithTimeout overrides the default bound on a single API call.
func WithTimeout(d time.Duration) Option {
	return func(a *Verifier) {
		a.timeout = d
	}
}

// WithHTTPClient makes the verifier send requests through the client, for
// example one with a custom transport or proxy. The timeout still applies
// per request.
func WithHTTPClient(c *http.Client) Option {
	return func(a *Verifier) {
		a.http = c
	}
}

// New creates a VIES verifier.
func New(opts ...Option) *Verifier {
	a := &Verifier{
		baseURL: DefaultBaseURL,
		timeout: DefaultTimeout,
	}
	for _, opt := range opts {
		opt(a)
	}
	if u, err := url.Parse(a.baseURL); err != nil {
		a.initErr = tin.ErrInput.WithMsgf("invalid base URL %q", a.baseURL).WithCause(err)
	} else if u.Scheme != "http" && u.Scheme != "https" {
		a.initErr = tin.ErrInput.WithMsgf("invalid base URL %q: scheme must be http or https", a.baseURL)
	} else if u.Hostname() == "" {
		a.initErr = tin.ErrInput.WithMsgf("invalid base URL %q: missing host", a.baseURL)
	}
	if a.http != nil {
		a.conn = resty.NewWithClient(a.http)
	} else {
		a.conn = resty.New()
	}
	a.conn.
		SetBaseURL(a.baseURL).
		SetTimeout(a.timeout).
		SetHeader("User-Agent", userAgent)
	return a
}

// checkVatRequest is the request body for the VIES API.
type checkVatRequest struct {
	CountryCode l10n.TaxCountryCode `json:"countryCode"`
	VatNumber   cbc.Code            `json:"vatNumber"`
}

// errorResponse is the body VIES returns on failure statuses.
type errorResponse struct {
	Message string `json:"message"`
}

// checkVatResponse is the successful response from a VAT number check. Valid
// is a pointer so that an absent field is distinguishable from an explicit
// false: a body without it is not an answer. The address is ignored: VIES
// returns it as one unstructured string, which cannot honestly become a
// structured GOBL address.
type checkVatResponse struct {
	Valid       *bool               `json:"valid"`
	Name        string              `json:"name"`
	CountryCode l10n.TaxCountryCode `json:"countryCode"`
	VatNumber   cbc.Code            `json:"vatNumber"`
}

// Source names VIES as the register.
func (a *Verifier) Source() cbc.Key {
	return Source
}

// Supports reports whether the identifier is a tax identity of a country
// that VIES covers. Identities, typed or not, never route here.
func (a *Verifier) Supports(id tin.Identifier) bool {
	return id.IsTaxID() && id.Country.Code().In(countryCodes...)
}

// Verify checks the request's identifier against VIES. VIES answers from the
// code alone, so the party is not read. An unregistered number is an Answer
// with StatusInvalid, not an error.
func (a *Verifier) Verify(ctx context.Context, req tin.Request) (*tin.Answer, error) {
	id := req.Identifier
	out, err := a.checkVat(ctx, id.Country, id.Code)
	if err != nil {
		return nil, err
	}
	ans := &tin.Answer{
		Status:    tin.StatusInvalid,
		TaxID:     confirmedTaxID(id.Country, id.Code, out),
		CheckedAt: time.Now().UTC(),
	}
	if *out.Valid {
		ans.Status = tin.StatusValid
	}
	if name := unmask(out.Name); name != "" {
		ans.Record = &tin.Record{Name: name}
	}
	return ans, nil
}

// checkVat posts the number to VIES and decodes the answer. The response
// always carries a validity field on return.
func (a *Verifier) checkVat(ctx context.Context, country l10n.TaxCountryCode, code cbc.Code) (*checkVatResponse, error) {
	if a.initErr != nil {
		return nil, a.initErr
	}
	if country == "" {
		return nil, tin.ErrInput.WithMessage("no country code provided")
	}
	if code == "" {
		return nil, tin.ErrInput.WithMessage("no tax ID code provided")
	}

	reqBody := checkVatRequest{
		CountryCode: country,
		VatNumber:   code,
	}

	resp, err := a.conn.R().
		SetContext(ctx).
		SetBody(reqBody).
		Post(checkVatPath)
	if err != nil {
		return nil, tin.ErrNetwork.WithCause(err)
	}

	if !resp.IsSuccess() {
		return nil, statusError(resp)
	}

	status := strconv.Itoa(resp.StatusCode())
	out := new(checkVatResponse)
	if err := json.Unmarshal(resp.Body(), out); err != nil {
		return nil, tin.ErrNetwork.WithCode(status).WithMessage("decoding response").WithCause(err)
	}
	if out.Valid == nil {
		return nil, tin.ErrNetwork.WithCode(status).WithMessage("response carries no validity field")
	}
	return out, nil
}

// confirmedTaxID builds the tax identity as VIES confirmed it, preferring the
// response's echo of country and number and falling back to the request.
func confirmedTaxID(country l10n.TaxCountryCode, code cbc.Code, resp *checkVatResponse) *tax.Identity {
	out := &tax.Identity{Country: resp.CountryCode, Code: resp.VatNumber}
	if out.Country == "" {
		out.Country = country
	}
	if out.Code == "" {
		out.Code = code
	}
	return out
}

// statusError maps a non-2xx response onto the tin error taxonomy.
func statusError(resp *resty.Response) error {
	code := resp.StatusCode()
	status := strconv.Itoa(code)
	msg := errorMessage(resp.Body())

	switch code {
	case http.StatusBadRequest:
		// VIES answers 400 when the request itself is malformed, for example
		// a number with symbols in it. That is a problem with the input, not
		// with the register.
		return tin.ErrInput.WithCode(status).WithMessage(msg)
	case http.StatusTooManyRequests:
		return &tin.RateLimitedError{RetryAfter: retryAfter(resp.Header())}
	default:
		// VIES has no auth and no per-resource statuses, so everything else,
		// including edge responses such as 403, is the register failing to
		// answer.
		return tin.ErrServer.WithCode(status).WithMessage(msg)
	}
}

// errorMessage extracts a readable message from the VIES error body, falling
// back to the raw body when it is not the expected shape.
func errorMessage(body []byte) string {
	out := new(errorResponse)
	if err := json.Unmarshal(body, out); err == nil && out.Message != "" {
		return out.Message
	}
	const maxRaw = 200
	if len(body) > maxRaw {
		// Cutting on a byte boundary can split a multibyte rune; drop the
		// partial sequence rather than emit invalid UTF-8.
		return strings.ToValidUTF8(string(body[:maxRaw]), "")
	}
	return string(body)
}

// retryAfter works out how long to wait before retrying, preferring the
// standard Retry-After header and falling back to a fixed default. The
// header grammar allows both delay seconds (including zero) and an
// HTTP-date; a date in the past clamps to zero.
func retryAfter(h http.Header) time.Duration {
	v := h.Get("Retry-After")
	if v == "" {
		return defaultRetryAfter
	}
	if secs, err := strconv.ParseInt(v, 10, 64); err == nil && secs >= 0 {
		// Clamp before multiplying: a huge delay would overflow into a
		// negative duration and cause an immediate requeue.
		const maxSecs = int64(math.MaxInt64) / int64(time.Second)
		return time.Duration(min(secs, maxSecs)) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		return max(time.Until(t), 0)
	}
	return defaultRetryAfter
}

// unmask cleans a VIES text field. Member states that do not disclose trader
// details answer with "---", which callers should see as empty rather than as
// a literal name or address.
func unmask(s string) string {
	s = strings.TrimSpace(s)
	if s == "---" {
		return ""
	}
	return s
}
