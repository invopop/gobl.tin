// Package dgfip implements the verifier for French VAT numbers. The DGFiP,
// the French tax administration, publishes every active French VAT number as
// open data; the Annuaire des Entreprises gives the registered name and the
// structured address of the company.
package dgfip

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// Source names the DGFiP as the register in checks. The DGFiP decides
// whether a French VAT number is valid, whichever service answers.
const Source cbc.Key = "dgfip"

// DefaultDatasetURL is the dataset "Numéros de TVA intracommunautaire
// français" on data.economie.gouv.fr. It holds every active French VAT number
// and is updated on working days.
const DefaultDatasetURL = "https://data.economie.gouv.fr/api/explore/v2.1/catalog/datasets/numeros-de-tva-intracommunautaires-francais"

// DefaultCompaniesURL is the API of the Annuaire des Entreprises (API
// Recherche d'entreprises), which gives the Sirene name and address.
const DefaultCompaniesURL = "https://recherche-entreprises.api.gouv.fr"

// DefaultTimeout bounds a single API call.
const DefaultTimeout = 15 * time.Second

// userAgent identifies this library to the services.
const userAgent = "gobl.tin (+https://github.com/invopop/gobl.tin)"

// defaultRetryAfter is used when a service rate limits without saying when
// to retry.
const defaultRetryAfter = time.Minute

// notDisclosed is the value the Annuaire des Entreprises gives to every field
// of a person or establishment that opted out of publication in Sirene.
const notDisclosed = "[NON-DIFFUSIBLE]"

// vatCode is a French VAT number without the country: a two character key
// and the SIREN. It also keeps user input out of the dataset query.
var vatCode = regexp.MustCompile(`^[0-9A-Z]{2}\d{9}$`)

// Verifier is the DGFiP verifier. It implements tin.Verifier.
type Verifier struct {
	datasetURL   string
	companiesURL string
	timeout      time.Duration
	http         *http.Client
	dataset      *resty.Client
	companies    *resty.Client

	// initErr records an invalid construction, such as an unparseable base
	// URL. Verify returns it, so a misconfiguration surfaces on the first call.
	initErr error
}

// Option configures the Verifier.
type Option func(*Verifier)

// WithDatasetURL points the dataset queries at a different endpoint. It
// exists so that tests can run against a local server.
func WithDatasetURL(u string) Option {
	return func(v *Verifier) {
		v.datasetURL = u
	}
}

// WithCompaniesURL points the company queries at a different endpoint. It
// exists so that tests can run against a local server.
func WithCompaniesURL(u string) Option {
	return func(v *Verifier) {
		v.companiesURL = u
	}
}

// WithTimeout overrides the default bound on a single API call.
func WithTimeout(d time.Duration) Option {
	return func(v *Verifier) {
		v.timeout = d
	}
}

// WithHTTPClient makes the verifier send requests through the client. The
// timeout still applies per request.
func WithHTTPClient(c *http.Client) Option {
	return func(v *Verifier) {
		v.http = c
	}
}

// New creates a DGFiP verifier.
func New(opts ...Option) *Verifier {
	v := &Verifier{
		datasetURL:   DefaultDatasetURL,
		companiesURL: DefaultCompaniesURL,
		timeout:      DefaultTimeout,
	}
	for _, opt := range opts {
		opt(v)
	}
	for _, u := range []string{v.datasetURL, v.companiesURL} {
		if err := checkBaseURL(u); err != nil {
			v.initErr = err
		}
	}
	v.dataset = v.client(v.datasetURL)
	v.companies = v.client(v.companiesURL)
	return v
}

func checkBaseURL(raw string) error {
	u, err := url.Parse(raw)
	switch {
	case err != nil:
		return tin.ErrServer.WithMsgf("invalid base URL %q", raw).WithCause(err)
	case u.Scheme != "http" && u.Scheme != "https":
		return tin.ErrServer.WithMsgf("invalid base URL %q: scheme must be http or https", raw)
	case u.Hostname() == "":
		return tin.ErrServer.WithMsgf("invalid base URL %q: missing host", raw)
	}
	return nil
}

func (v *Verifier) client(base string) *resty.Client {
	var c *resty.Client
	if v.http != nil {
		// A copy, so that setting the timeout leaves the caller's client
		// as it was.
		hc := *v.http
		c = resty.NewWithClient(&hc)
	} else {
		c = resty.New()
	}
	return c.SetBaseURL(base).SetTimeout(v.timeout).SetHeader("User-Agent", userAgent)
}

// Source names the DGFiP as the register.
func (v *Verifier) Source() cbc.Key {
	return Source
}

// Supports reports whether the identifier is a French tax identity.
func (v *Verifier) Supports(id tin.Identifier) bool {
	return id.Country == l10n.FR.Tax()
}

// Verify checks a French VAT number. A number in the dataset is valid, and
// gets the name and the address of the company. For a number it does not
// hold, an active company with that SIREN is valid with the status
// not-vat-registered, such as a business under the franchise en base;
// otherwise the number is invalid. Any failure is an error, so that the next
// verifier answers.
func (v *Verifier) Verify(ctx context.Context, req tin.Request) (*tin.Answer, error) {
	if v.initErr != nil {
		return nil, v.initErr
	}
	code := req.Identifier.Code
	if code == "" {
		return nil, tin.ErrInput.WithMessage("no tax ID code provided")
	}
	if !vatCode.MatchString(code.String()) {
		return nil, tin.ErrInput.WithMessage("a French VAT number has a key of 2 characters and a SIREN of 9 digits")
	}
	found, err := v.inDataset(ctx, code)
	if err != nil {
		return nil, err
	}
	c, err := v.company(ctx, code.String()[2:])
	if err != nil {
		return nil, err
	}
	ans := &tin.Answer{Status: tin.StatusValid, TaxID: taxID(code), CheckedAt: time.Now().UTC()}
	switch {
	case found:
		ans.Record = c.record()
	case c != nil && c.AdminStatus == "A":
		ans.Record = c.record()
		if ans.Record == nil {
			ans.Record = new(tin.Record)
		}
		ans.Record.Status = tin.RecordStatusNotVATRegistered
	default:
		ans.Status = tin.StatusInvalid
	}
	return ans, nil
}

func taxID(code cbc.Code) *tax.Identity {
	return &tax.Identity{Country: l10n.FR.Tax(), Code: code}
}

// datasetResponse is the answer of the dataset records query.
type datasetResponse struct {
	TotalCount *int `json:"total_count"`
}

// inDataset reports whether the dataset holds the VAT number.
func (v *Verifier) inDataset(ctx context.Context, code cbc.Code) (bool, error) {
	resp, err := v.dataset.R().
		SetContext(ctx).
		SetQueryParams(map[string]string{
			"where":  `vat_no="` + code.String() + `"`,
			"select": "vat_no",
			"limit":  "1",
		}).
		Get("/records")
	if err != nil {
		return false, tin.ErrNetwork.WithCause(err)
	}
	if !resp.IsSuccess() {
		return false, statusError(resp, "dataset")
	}
	out := new(datasetResponse)
	if err := json.Unmarshal(resp.Body(), out); err != nil || out.TotalCount == nil {
		return false, tin.ErrNetwork.WithCode(strconv.Itoa(resp.StatusCode())).WithMessage("decoding dataset response").WithCause(err)
	}
	return *out.TotalCount > 0, nil
}

// companiesResponse is the answer of the Annuaire des Entreprises search.
type companiesResponse struct {
	// Nil when the response has no results array.
	Results *[]company `json:"results"`
}

type company struct {
	SIREN       string `json:"siren"`
	FullName    string `json:"nom_complet"`
	LegalName   string `json:"nom_raison_sociale"`
	AdminStatus string `json:"etat_administratif"`
	Complements struct {
		SoleTrader bool `json:"est_entrepreneur_individuel"`
	} `json:"complements"`
	HeadOffice establishment `json:"siege"`
}

type establishment struct {
	Number          string `json:"numero_voie"`
	Repetition      string `json:"indice_repetition"`
	StreetType      string `json:"type_voie"`
	StreetName      string `json:"libelle_voie"`
	Complement      string `json:"complement_adresse"`
	SpecialDelivery string `json:"distribution_speciale"`
	PostCode        string `json:"code_postal"`
	Locality        string `json:"libelle_commune"`
	Cedex           string `json:"cedex"`
	CedexLocality   string `json:"libelle_cedex"`
	ForeignCountry  string `json:"code_pays_etranger"`
}

// company returns the company with the SIREN, or nil when the Annuaire des
// Entreprises does not list it.
func (v *Verifier) company(ctx context.Context, siren string) (*company, error) {
	resp, err := v.companies.R().
		SetContext(ctx).
		SetQueryParams(map[string]string{"q": siren, "per_page": "5"}).
		Get("/search")
	if err != nil {
		return nil, tin.ErrNetwork.WithCause(err)
	}
	if !resp.IsSuccess() {
		return nil, statusError(resp, "company search")
	}
	out := new(companiesResponse)
	if err := json.Unmarshal(resp.Body(), out); err != nil || out.Results == nil {
		return nil, tin.ErrNetwork.WithCode(strconv.Itoa(resp.StatusCode())).WithMessage("decoding company response").WithCause(err)
	}
	for i := range *out.Results {
		if c := &(*out.Results)[i]; c.SIREN == siren {
			return c, nil
		}
	}
	return nil, nil
}

// record maps the company onto a record, or nil for no company. Fields that
// Sirene does not publish are left empty, so a person who opted out gets no
// name or address.
func (c *company) record() *tin.Record {
	if c == nil {
		return nil
	}
	rec := &tin.Record{Name: c.name(), Address: c.HeadOffice.address()}
	if c.AdminStatus == "C" {
		rec.Status = tin.RecordStatusInactive
	}
	if rec.Name == "" && rec.Address == nil && rec.Status == "" {
		return nil
	}
	return rec
}

// name is the legal name of a company. A sole trader has none, so the name
// of the person is the full name up to the brackets, which hold a usage or
// trade name: "FRANCK DELHAYE (FRANCK DELHAYE)" gives "FRANCK DELHAYE".
func (c *company) name() string {
	if c.Complements.SoleTrader || c.LegalName == "" {
		name, _, _ := strings.Cut(c.FullName, " (")
		return disclosed(name)
	}
	return disclosed(c.LegalName)
}

// address maps the head office onto a GOBL address. It gives no address for
// an establishment abroad, without a postcode or CEDEX, or with an undisclosed
// field.
func (e *establishment) address() *org.Address {
	fields := []string{e.Number, e.Repetition, e.StreetType, e.StreetName, e.Complement, e.SpecialDelivery, e.PostCode, e.Locality, e.Cedex, e.CedexLocality}
	for _, f := range fields {
		if strings.TrimSpace(f) == notDisclosed {
			return nil
		}
	}
	if e.ForeignCountry != "" || strings.TrimSpace(e.PostCode+e.Cedex) == "" {
		return nil
	}
	out := &org.Address{
		Number:        strings.TrimSpace(e.Number + e.Repetition),
		Street:        strings.Join(strings.Fields(e.StreetType+" "+e.StreetName), " "),
		StreetExtra:   strings.TrimSpace(e.Complement),
		PostOfficeBox: strings.TrimSpace(e.SpecialDelivery),
		Code:          cbc.Code(strings.TrimSpace(e.PostCode)),
		Locality:      strings.TrimSpace(e.Locality),
		Country:       l10n.ISOCountryCode("FR"),
	}
	if c := strings.TrimSpace(e.Cedex); c != "" {
		// A CEDEX code replaces the postcode for mail to large senders.
		out.Code = cbc.Code(c)
		if l := strings.TrimSpace(e.CedexLocality); l != "" {
			out.Locality = l
		}
	}
	return out
}

func disclosed(s string) string {
	s = strings.TrimSpace(s)
	if s == notDisclosed {
		return ""
	}
	return s
}

// statusError maps a non-2xx response onto the tin error taxonomy. The
// services take no input from the party but the number, which is checked
// first, so a 4xx other than 429 is the service failing.
func statusError(resp *resty.Response, what string) error {
	status := strconv.Itoa(resp.StatusCode())
	if resp.StatusCode() == http.StatusTooManyRequests {
		return &tin.RateLimitedError{RetryAfter: retryAfter(resp.Header()), Reason: what}
	}
	return tin.ErrServer.WithCode(status).WithMessage(what + ": " + clamp(string(resp.Body())))
}

// retryAfter works out how long to wait before retrying: the standard
// Retry-After header, or the reset time of the data.economie.gouv.fr quota,
// or a fixed default.
func retryAfter(h http.Header) time.Duration {
	if v := h.Get("Retry-After"); v != "" {
		if secs, err := strconv.ParseInt(v, 10, 64); err == nil && secs >= 0 {
			const maxSecs = int64(math.MaxInt64) / int64(time.Second)
			return time.Duration(min(secs, maxSecs)) * time.Second
		}
		if t, err := http.ParseTime(v); err == nil {
			return max(time.Until(t), 0)
		}
	}
	if v := h.Get("X-RateLimit-Reset"); v != "" {
		if t, err := time.Parse("2006-01-02 15:04:05-07:00", v); err == nil {
			return max(time.Until(t), 0)
		}
	}
	return defaultRetryAfter
}

// maxMessage bounds any text taken from a response into an error.
const maxMessage = 200

func clamp(s string) string {
	if len(s) > maxMessage {
		s = s[:maxMessage]
	}
	return strings.ToValidUTF8(s, "")
}
