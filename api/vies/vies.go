// Package vies implements the TIN lookup against the VIES service, the
// European Commission's VAT number registry.
package vies

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/invopop/gobl.tin/api"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/tax"
)

// Source identifies VIES as the registry in lookup results.
const Source cbc.Key = "vies"

// DefaultBaseURL is the production VIES REST endpoint. It is the default
// because VIES lookups are read-only and have no side effects, and the
// Commission provides no test environment, so lookups always run against
// production.
const DefaultBaseURL = "https://ec.europa.eu/taxation_customs/vies/rest-api"

// DefaultTimeout bounds a single API call.
const DefaultTimeout = 15 * time.Second

// userAgent identifies this library to the Commission's edge, which has been
// seen blocking clients that send default Go user agents.
const userAgent = "gobl.tin (+https://github.com/invopop/gobl.tin)"

const checkVatPath = "/check-vat-number"

// defaultRetryAfter is used when VIES rate limits without a Retry-After
// header. VIES does not document its limits, so this is a polite guess.
const defaultRetryAfter = time.Minute

// API implements the VIES lookup.
type API struct {
	baseURL string
	timeout time.Duration
	conn    *resty.Client
}

// Option configures the API.
type Option func(*API)

// WithBaseURL points the API at a different endpoint. It exists so that tests
// can run against a local server instead of the live VIES service.
func WithBaseURL(url string) Option {
	return func(a *API) {
		a.baseURL = url
	}
}

// WithTimeout overrides the default bound on a single API call.
func WithTimeout(d time.Duration) Option {
	return func(a *API) {
		a.timeout = d
	}
}

// New creates a new VIES API client.
func New(opts ...Option) *API {
	a := &API{
		baseURL: DefaultBaseURL,
		timeout: DefaultTimeout,
	}
	for _, opt := range opts {
		opt(a)
	}
	a.conn = resty.New().
		SetBaseURL(a.baseURL).
		SetTimeout(a.timeout).
		SetHeader("User-Agent", userAgent)
	return a
}

// HTTPClient returns the underlying HTTP client, which lets tests attach a
// mock transport.
func (a *API) HTTPClient() *http.Client {
	return a.conn.GetClient()
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

// checkVatResponse is the successful response from a VAT number check.
type checkVatResponse struct {
	Valid   bool   `json:"valid"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

// LookupTIN checks the VAT number against VIES. An unregistered number is a
// Result with Valid false, not an error.
func (a *API) LookupTIN(ctx context.Context, tid *tax.Identity) (*api.Result, error) {
	reqBody := checkVatRequest{
		CountryCode: tid.Country,
		VatNumber:   tid.Code,
	}

	resp, err := a.conn.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(reqBody).
		Post(checkVatPath)
	if err != nil {
		return nil, api.ErrNetwork.WithCause(err)
	}

	if !resp.IsSuccess() {
		return nil, statusError(resp)
	}

	var out checkVatResponse
	if err := json.Unmarshal(resp.Body(), &out); err != nil {
		return nil, api.ErrNetwork.WithMessage("decoding response").WithCause(err)
	}

	return &api.Result{
		Valid:   out.Valid,
		Name:    unmask(out.Name),
		Address: unmask(out.Address),
		Source:  Source,
	}, nil
}

// statusError maps a non-2xx response onto the api error taxonomy.
func statusError(resp *resty.Response) error {
	code := resp.StatusCode()
	msg := errorMessage(resp.Body(), code)

	switch code {
	case http.StatusBadRequest:
		// VIES answers 400 when the request itself is malformed, for example
		// a number with symbols in it. That is a problem with the input, not
		// with the network.
		return api.ErrInput.WithMessage(msg)
	case http.StatusTooManyRequests:
		return &api.RateLimitedError{RetryAfter: retryAfter(resp.Header())}
	default:
		return api.ErrNetwork.WithMessage(msg)
	}
}

// errorMessage extracts a readable message from the VIES error body, falling
// back to the status code when the body is not the expected shape.
func errorMessage(body []byte, code int) string {
	out := new(errorResponse)
	if err := json.Unmarshal(body, out); err == nil && out.Message != "" {
		return "received " + strconv.Itoa(code) + " status code: " + out.Message
	}
	return "received " + strconv.Itoa(code) + " status code with unknown body"
}

// retryAfter works out how long to wait before retrying, preferring the
// standard Retry-After header and falling back to a fixed default.
func retryAfter(h http.Header) time.Duration {
	if v := h.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
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
