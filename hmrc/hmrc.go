// Package hmrc implements the verifier for the VAT register of HMRC, the UK
// tax authority, through its "Check a UK VAT number" API, version 2.
package hmrc

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// Source names HMRC as the register in checks.
const Source cbc.Key = "hmrc"

// DefaultBaseURL is the HMRC production API.
const DefaultBaseURL = "https://api.service.hmrc.gov.uk"

// SandboxBaseURL is the HMRC sandbox API. It holds only HMRC's mock VAT
// numbers, and it needs the credentials of a sandbox application.
const SandboxBaseURL = "https://test-api.service.hmrc.gov.uk"

// DefaultTimeout bounds a single API call.
const DefaultTimeout = 15 * time.Second

// userAgent identifies this library to HMRC.
const userAgent = "gobl.tin (+https://github.com/invopop/gobl.tin)"

const (
	tokenPath  = "/oauth/token"
	lookupPath = "/organisations/vat/check-vat-number/lookup/{vrn}"

	// acceptV2 selects version 2 of the API. HMRC removed version 1 on
	// 17 February 2025.
	acceptV2 = "application/vnd.hmrc.2.0+json"

	// scope is the OAuth scope of the lookup.
	scope = "read:vat"
)

// tokenMargin renews a token this long before it expires, so that a token
// does not expire during a request.
const tokenMargin = time.Minute

// defaultRetryAfter is used when HMRC rate limits without a Retry-After
// header. HMRC does not publish its limits, so this is a polite guess.
const defaultRetryAfter = time.Minute

// codeNotFound is the error code of a lookup for a VAT number that HMRC
// does not hold.
const codeNotFound = "NOT_FOUND"

// vrnPattern matches the VAT numbers that HMRC checks: 9 digits, or 12 for a
// branch. The GD and HA codes of government departments and health
// authorities are valid UK VAT numbers, but HMRC does not check them.
var vrnPattern = regexp.MustCompile(`^(\d{9}|\d{12})$`)

// Verifier is the HMRC verifier. It implements tin.Verifier.
type Verifier struct {
	baseURL      string
	clientID     string
	clientSecret string
	timeout      time.Duration
	http         *http.Client
	conn         *resty.Client

	// mu guards the token, which all requests share until it expires.
	mu      sync.Mutex
	token   string
	expires time.Time

	// initErr records an invalid construction, such as missing credentials
	// or an unparseable base URL. Verify returns it, so a misconfiguration
	// surfaces on the first call.
	initErr error
}

// Option configures the Verifier.
type Option func(*Verifier)

// WithSandbox sends requests to SandboxBaseURL.
func WithSandbox() Option {
	return func(v *Verifier) {
		v.baseURL = SandboxBaseURL
	}
}

// WithBaseURL points the API at a different endpoint. It exists so that tests
// can run against a local server instead of the live HMRC service.
func WithBaseURL(url string) Option {
	return func(v *Verifier) {
		v.baseURL = url
	}
}

// WithTimeout overrides the default bound on a single API call.
func WithTimeout(d time.Duration) Option {
	return func(v *Verifier) {
		v.timeout = d
	}
}

// WithHTTPClient makes the verifier send requests through the client, for
// example one with a custom transport or proxy. The timeout still applies
// per request.
func WithHTTPClient(c *http.Client) Option {
	return func(v *Verifier) {
		v.http = c
	}
}

// New creates an HMRC verifier with the credentials of an application on
// the HMRC Developer Hub that is subscribed to "Check a UK VAT number". The
// verifier gets an OAuth token with the credentials and reuses it until it
// expires.
func New(clientID, clientSecret string, opts ...Option) *Verifier {
	v := &Verifier{
		baseURL:      DefaultBaseURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		timeout:      DefaultTimeout,
	}
	for _, opt := range opts {
		opt(v)
	}
	if clientID == "" || clientSecret == "" {
		// Missing credentials are a problem with the configuration, not with
		// the identifier, so they are the register failing to answer.
		v.initErr = tin.ErrServer.WithMessage("no client credentials provided")
	}
	if u, err := url.Parse(v.baseURL); err != nil {
		v.initErr = tin.ErrInput.WithMsgf("invalid base URL %q", v.baseURL).WithCause(err)
	} else if u.Scheme != "http" && u.Scheme != "https" {
		v.initErr = tin.ErrInput.WithMsgf("invalid base URL %q: scheme must be http or https", v.baseURL)
	} else if u.Hostname() == "" {
		v.initErr = tin.ErrInput.WithMsgf("invalid base URL %q: missing host", v.baseURL)
	}
	if v.http != nil {
		// A copy, so that setting the timeout leaves the caller's client
		// as it was.
		hc := *v.http
		v.conn = resty.NewWithClient(&hc)
	} else {
		v.conn = resty.New()
	}
	v.conn.
		SetBaseURL(v.baseURL).
		SetTimeout(v.timeout).
		SetHeader("User-Agent", userAgent)
	return v
}

// tokenResponse is the OAuth token response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

// lookupResponse is the response for a VAT number that HMRC holds.
type lookupResponse struct {
	Target *struct {
		Name      string   `json:"name"`
		VATNumber string   `json:"vatNumber"`
		Address   *address `json:"address"`
	} `json:"target"`
	ProcessingDate string `json:"processingDate"`
}

// address is the address that HMRC holds. Only line1 and countryCode are
// always present.
type address struct {
	Line1       string `json:"line1"`
	Line2       string `json:"line2"`
	Line3       string `json:"line3"`
	Line4       string `json:"line4"`
	Line5       string `json:"line5"`
	Postcode    string `json:"postcode"`
	CountryCode string `json:"countryCode"`
}

// errorResponse is the body HMRC returns on failure statuses.
type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Source names HMRC as the register.
func (v *Verifier) Source() cbc.Key {
	return Source
}

// Supports reports whether the identifier is a UK VAT number that HMRC
// checks. Northern Ireland (XI) VAT numbers are in VIES, not here.
func (v *Verifier) Supports(id tin.Identifier) bool {
	return id.Country == l10n.GB.Tax() && vrnPattern.MatchString(id.Code.String())
}

// Verify looks up the request's VAT number at HMRC. HMRC answers from the
// number alone, so the party is not read. A number that HMRC does not hold
// is an Answer with StatusInvalid, not an error.
func (v *Verifier) Verify(ctx context.Context, req tin.Request) (*tin.Answer, error) {
	if v.initErr != nil {
		return nil, v.initErr
	}
	id := req.Identifier
	if !vrnPattern.MatchString(id.Code.String()) {
		return nil, tin.ErrInput.WithMsgf("HMRC checks only VAT numbers of 9 or 12 digits, not %q", clamp(id.Code.String()))
	}

	resp, err := v.lookup(ctx, id.Code)
	if err != nil {
		return nil, err
	}
	status := strconv.Itoa(resp.StatusCode())
	if resp.StatusCode() == http.StatusNotFound {
		out := new(errorResponse)
		if json.Unmarshal(resp.Body(), out) == nil && out.Code == codeNotFound {
			return &tin.Answer{
				Status:    tin.StatusInvalid,
				TaxID:     &tax.Identity{Country: l10n.GB.Tax(), Code: id.Code},
				CheckedAt: time.Now().UTC(),
			}, nil
		}
		// Any other 404, such as an unknown path at HMRC's gateway, is not
		// an answer about the number.
		return nil, statusError(resp)
	}
	if !resp.IsSuccess() {
		return nil, statusError(resp)
	}

	out := new(lookupResponse)
	if err := json.Unmarshal(resp.Body(), out); err != nil {
		return nil, tin.ErrNetwork.WithCode(status).WithMessage("decoding response").WithCause(err)
	}
	if out.Target == nil {
		return nil, tin.ErrNetwork.WithCode(status).WithMessage("response carries no target")
	}
	ans := &tin.Answer{
		Status:    tin.StatusValid,
		TaxID:     &tax.Identity{Country: l10n.GB.Tax(), Code: cbc.Code(strings.TrimSpace(out.Target.VATNumber))},
		CheckedAt: processingDate(out.ProcessingDate),
	}
	if ans.TaxID.Code == "" {
		ans.TaxID.Code = id.Code
	}
	rec := &tin.Record{
		Name:    strings.TrimSpace(out.Target.Name),
		Address: out.Target.Address.toOrg(),
	}
	if rec.Name != "" || rec.Address != nil {
		ans.Record = rec
	}
	return ans, nil
}

// lookup sends the lookup request. When HMRC rejects the token, which it
// can revoke before it expires, the verifier gets a new one and tries once
// more.
func (v *Verifier) lookup(ctx context.Context, vrn cbc.Code) (*resty.Response, error) {
	var resp *resty.Response
	for attempt := 0; attempt < 2; attempt++ {
		token, err := v.accessToken(ctx)
		if err != nil {
			return nil, err
		}
		resp, err = v.conn.R().
			SetContext(ctx).
			SetHeader("Accept", acceptV2).
			SetAuthToken(token).
			SetPathParam("vrn", vrn.String()).
			Get(lookupPath)
		if err != nil {
			return nil, tin.ErrNetwork.WithCause(err)
		}
		if resp.StatusCode() != http.StatusUnauthorized {
			break
		}
		v.dropToken(token)
	}
	return resp, nil
}

// accessToken returns the shared token, and gets a new one when there is
// none or it is about to expire.
func (v *Verifier) accessToken(ctx context.Context) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.token != "" && time.Now().Before(v.expires) {
		return v.token, nil
	}

	resp, err := v.conn.R().
		SetContext(ctx).
		SetFormData(map[string]string{
			"client_id":     v.clientID,
			"client_secret": v.clientSecret,
			"grant_type":    "client_credentials",
			"scope":         scope,
		}).
		Post(tokenPath)
	if err != nil {
		return "", tin.ErrNetwork.WithCause(err)
	}
	status := strconv.Itoa(resp.StatusCode())
	if !resp.IsSuccess() {
		if resp.StatusCode() == http.StatusTooManyRequests {
			return "", &tin.RateLimitedError{RetryAfter: retryAfter(resp.Header())}
		}
		// Wrong credentials are a configuration problem, not a problem
		// with the identifier, so they are the register failing to answer.
		return "", tin.ErrServer.WithCode(status).WithMessage("getting a token: " + errorMessage(resp.Body()))
	}
	out := new(tokenResponse)
	if err := json.Unmarshal(resp.Body(), out); err != nil {
		return "", tin.ErrNetwork.WithCode(status).WithMessage("decoding token").WithCause(err)
	}
	if out.AccessToken == "" {
		return "", tin.ErrNetwork.WithCode(status).WithMessage("token response carries no token")
	}
	v.token = out.AccessToken
	v.expires = time.Now().Add(time.Duration(out.ExpiresIn)*time.Second - tokenMargin)
	return v.token, nil
}

// dropToken forgets the token, unless another request already replaced it.
func (v *Verifier) dropToken(token string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.token == token {
		v.token = ""
	}
}

// toOrg maps the HMRC address onto a GOBL address. HMRC gives the street
// part as free lines. The first line is the street. When there are more
// lines, the last one is the post town, which is the usual order of a UK
// address, and the lines between them are the extra street part.
func (a *address) toOrg() *org.Address {
	if a == nil {
		return nil
	}
	var lines []string
	for _, l := range []string{a.Line1, a.Line2, a.Line3, a.Line4, a.Line5} {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	out := &org.Address{
		Code:    cbc.Code(strings.TrimSpace(a.Postcode)),
		Country: l10n.ISOCountryCode(strings.ToUpper(strings.TrimSpace(a.CountryCode))),
	}
	switch len(lines) {
	case 0:
	case 1:
		out.Street = lines[0]
	default:
		out.Street = lines[0]
		out.StreetExtra = strings.Join(lines[1:len(lines)-1], ", ")
		out.Locality = lines[len(lines)-1]
	}
	if out.Street == "" && out.Code == "" {
		return nil
	}
	return out
}

// processingDate parses the time of the answer that HMRC gives. It is when
// HMRC answered, so an unreadable value falls back to now.
func processingDate(s string) time.Time {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
	if err != nil {
		return time.Now().UTC()
	}
	return t.UTC()
}

// statusError maps a failure status onto the tin error taxonomy.
func statusError(resp *resty.Response) error {
	code := resp.StatusCode()
	status := strconv.Itoa(code)
	msg := errorMessage(resp.Body())
	switch code {
	case http.StatusBadRequest:
		return tin.ErrInput.WithCode(status).WithMessage(msg)
	case http.StatusTooManyRequests:
		return &tin.RateLimitedError{RetryAfter: retryAfter(resp.Header())}
	default:
		// Everything else, including a 401 or 403 for an application that
		// HMRC does not accept, is the register failing to answer.
		return tin.ErrServer.WithCode(status).WithMessage(msg)
	}
}

// errorMessage extracts a readable message from the HMRC error body, falling
// back to the raw body when it is not the expected shape.
func errorMessage(body []byte) string {
	out := new(errorResponse)
	if err := json.Unmarshal(body, out); err == nil && out.Code != "" {
		if out.Message != "" {
			return clamp(out.Code + ": " + out.Message)
		}
		return clamp(out.Code)
	}
	return clamp(string(body))
}

// maxMessage bounds any text taken from an HMRC response into an error, so
// an untrusted body cannot make a failure arbitrarily large.
const maxMessage = 200

// clamp cuts s to maxMessage bytes and drops invalid UTF-8, so a message
// taken from an untrusted body is always valid text.
func clamp(s string) string {
	if len(s) > maxMessage {
		s = s[:maxMessage]
	}
	return strings.ToValidUTF8(s, "")
}

// retryAfter works out how long to wait before retrying, preferring the
// standard Retry-After header and falling back to a fixed default.
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
