// Package sirene implements the verifier for French SIREN identities. The
// Annuaire des Entreprises publishes the Sirene register of INSEE: the name,
// the administrative status and the structured address of each company.
package sirene

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
)

// Source names Sirene as the register in checks.
const Source cbc.Key = "sirene"

// IdentityType is the GOBL identity type of a SIREN.
const IdentityType cbc.Code = "SIREN"

// DefaultURL is the API of the Annuaire des Entreprises (API Recherche
// d'entreprises).
const DefaultURL = "https://recherche-entreprises.api.gouv.fr"

// DefaultTimeout bounds a single API call.
const DefaultTimeout = 15 * time.Second

const userAgent = "gobl.tin (+https://github.com/invopop/gobl.tin)"

const defaultRetryAfter = time.Minute

// notDisclosed is the value the Annuaire des Entreprises gives to every field
// of a person or establishment that opted out of publication in Sirene.
const notDisclosed = "[NON-DIFFUSIBLE]"

// sirenCode also keeps user input out of the search query.
var sirenCode = regexp.MustCompile(`^\d{9}$`)

// Verifier is the Sirene verifier. It implements tin.Verifier.
type Verifier struct {
	url     string
	timeout time.Duration
	http    *http.Client
	client  *resty.Client

	// initErr records an invalid construction, such as an unparseable base
	// URL. Verify returns it, so a misconfiguration surfaces on the first call.
	initErr error
}

// Option configures the Verifier.
type Option func(*Verifier)

// WithURL points the queries at a different endpoint. It exists so that tests
// can run against a local server.
func WithURL(u string) Option {
	return func(v *Verifier) {
		v.url = u
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

// New creates a Sirene verifier.
func New(opts ...Option) *Verifier {
	v := &Verifier{url: DefaultURL, timeout: DefaultTimeout}
	for _, opt := range opts {
		opt(v)
	}
	v.initErr = checkBaseURL(v.url)
	var c *resty.Client
	if v.http != nil {
		// A copy, so that setting the timeout leaves the caller's client
		// as it was.
		hc := *v.http
		c = resty.NewWithClient(&hc)
	} else {
		c = resty.New()
	}
	v.client = c.SetBaseURL(v.url).SetTimeout(v.timeout).SetHeader("User-Agent", userAgent)
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

// Source names Sirene as the register.
func (v *Verifier) Source() cbc.Key {
	return Source
}

// IdentityTypes lists the SIREN.
func (v *Verifier) IdentityTypes() []cbc.Code {
	return []cbc.Code{IdentityType}
}

// Supports reports whether the identifier is a SIREN identity, French or
// without a country.
func (v *Verifier) Supports(id tin.Identifier) bool {
	return id.Type == IdentityType && (id.Country == "" || id.Country == l10n.FR.Tax())
}

// Verify checks a SIREN. A company in Sirene is valid, and a closed one has
// the status dissolved. A SIREN that Sirene does not list is invalid. Any
// failure is an error.
func (v *Verifier) Verify(ctx context.Context, req tin.Request) (*tin.Answer, error) {
	code := req.Identifier.Code.String()
	if code == "" {
		return nil, tin.ErrInput.WithMessage("no SIREN provided")
	}
	c, err := v.Company(ctx, code)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return &tin.Answer{Status: tin.StatusInvalid, CheckedAt: time.Now().UTC()}, nil
	}
	rec := &tin.Record{Name: c.Name, Address: c.Address}
	if !c.Active {
		rec.Status = tin.RecordStatusDissolved
	}
	if rec.Name == "" && rec.Address == nil && rec.Status == "" {
		rec = nil
	}
	return &tin.Answer{Status: tin.StatusValid, Record: rec, CheckedAt: time.Now().UTC()}, nil
}

// Company is what Sirene publishes about a company. Fields that Sirene does
// not publish are empty, so a person who opted out has no name or address.
type Company struct {
	Name string

	// Structured head office address. Nil for a head office abroad.
	Address *org.Address

	// Active is false for a company that ceased trading.
	Active bool
}

// Company returns the company with the SIREN, or nil when the Annuaire des
// Entreprises does not list it. A SIREN without 9 digits is an input error.
func (v *Verifier) Company(ctx context.Context, siren string) (*Company, error) {
	if !sirenCode.MatchString(siren) {
		return nil, tin.ErrInput.WithMessage("a SIREN has 9 digits")
	}
	if v.initErr != nil {
		return nil, v.initErr
	}
	resp, err := v.client.R().
		SetContext(ctx).
		SetQueryParams(map[string]string{"q": siren, "per_page": "5"}).
		Get("/search")
	if err != nil {
		return nil, tin.ErrNetwork.WithCause(err)
	}
	if !resp.IsSuccess() {
		return nil, statusError(resp)
	}
	out := new(searchResponse)
	if err := json.Unmarshal(resp.Body(), out); err != nil || out.Results == nil {
		return nil, tin.ErrNetwork.WithCode(strconv.Itoa(resp.StatusCode())).WithMessage("decoding company response").WithCause(err)
	}
	for i := range *out.Results {
		if r := &(*out.Results)[i]; r.SIREN == siren {
			return &Company{Name: r.name(), Address: r.HeadOffice.address(), Active: r.AdminStatus == "A"}, nil
		}
	}
	return nil, nil
}

// searchResponse is the answer of the Annuaire des Entreprises search.
type searchResponse struct {
	// Nil when the response has no results array.
	Results *[]result `json:"results"`
}

type result struct {
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

// name is the legal name of a company. A sole trader has none, so the name
// of the person is the full name up to the brackets, which hold a usage or
// trade name: "FRANCK DELHAYE (FRANCK DELHAYE)" gives "FRANCK DELHAYE".
func (r *result) name() string {
	if r.Complements.SoleTrader || r.LegalName == "" {
		name, _, _ := strings.Cut(r.FullName, " (")
		return disclosed(name)
	}
	return disclosed(r.LegalName)
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
// service takes no input but the SIREN, which is checked first, so a 4xx
// other than 429 is the service failing.
func statusError(resp *resty.Response) error {
	if resp.StatusCode() == http.StatusTooManyRequests {
		return &tin.RateLimitedError{RetryAfter: retryAfter(resp.Header()), Reason: "company search"}
	}
	return tin.ErrServer.WithCode(strconv.Itoa(resp.StatusCode())).WithMessage("company search: " + clamp(string(resp.Body())))
}

// retryAfter reads the standard Retry-After header, or uses a fixed default.
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
