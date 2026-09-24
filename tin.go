// Package tin verifies the identifiers of GOBL parties against the registers
// that issue them, such as VIES for EU VAT numbers, and reports what the
// register holds so that a party can be corrected with a patch.
package tin

import (
	"context"

	"github.com/invopop/gobl.tin/api"
	"github.com/invopop/gobl.tin/api/vies"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// Verifier is one register client. Implement it to add a register.
type Verifier = api.Verifier

// Verification is the report for one party.
type Verification = api.Verification

// Check is the outcome for one identifier of a party.
type Check = api.Check

// Record is what the register holds for an identifier.
type Record = api.Record

// Mismatch is one field where the party disagrees with the record.
type Mismatch = api.Mismatch

// Status is the outcome of one check.
type Status = api.Status

// Check statuses.
const (
	StatusValid       = api.StatusValid
	StatusInvalid     = api.StatusInvalid
	StatusUnsupported = api.StatusUnsupported
	StatusUnverified  = api.StatusUnverified
)

// NameMatches reports whether two names agree once case, punctuation and
// surrounding whitespace are folded.
func NameMatches(a, b string) bool {
	return api.NameMatches(a, b)
}

// Client verifies the identifiers of a party with a set of verifiers.
//
// The Client holds no cache, so every call reaches the register. Caching, and
// how long an answer stays fresh, is the consuming application's decision.
// The verifiers are built once per Client and reused, so checks share
// connections.
type Client struct {
	viesOpts  []vies.Option
	injected  []api.Verifier
	verifiers []api.Verifier
}

// Option configures the Client.
type Option func(*Client)

// WithVIESOptions passes options through to the VIES verifier the Client
// builds, for example a timeout or a different base URL.
func WithVIESOptions(opts ...vies.Option) Option {
	return func(c *Client) {
		c.viesOpts = append(c.viesOpts, opts...)
	}
}

// WithVerifier adds a verifier. A verifier with the same Source as one the
// Client already holds takes its place; any other is appended after the
// default set.
func WithVerifier(v api.Verifier) Option {
	return func(c *Client) {
		c.injected = append(c.injected, v)
	}
}

// New creates a Client with the default verifier set: VIES.
func New(opts ...Option) *Client {
	c := new(Client)
	for _, opt := range opts {
		opt(c)
	}
	c.verifiers = []api.Verifier{vies.New(c.viesOpts...)}
	for _, v := range c.injected {
		c.verifiers = replaceOrAppend(c.verifiers, v)
	}
	c.injected = nil
	return c
}

// replaceOrAppend puts v in place of the verifier with the same Source, or
// at the end when none has it.
func replaceOrAppend(set []api.Verifier, v api.Verifier) []api.Verifier {
	for i, existing := range set {
		if existing.Source() == v.Source() {
			set[i] = v
			return set
		}
	}
	return append(set, v)
}

// Verify checks every identifier of the party: the tax identity first, then
// each identity in order. The report carries one Check per identifier. A
// register that cannot answer marks its Check unverified; Verify itself
// returns an error only for a nil party.
func (c *Client) Verify(ctx context.Context, party *org.Party) (*Verification, error) {
	if party == nil {
		return nil, ErrInput.WithMessage("no party provided")
	}
	ids := walk(party)
	report := api.NewVerification(party)
	for _, id := range ids {
		check := c.check(ctx, id)
		check.Mismatches = mismatches(party, ids, id, check)
		report.Checks = append(report.Checks, check)
	}
	return report, nil
}

// VerifyTaxID checks one tax identity. An identity that no verifier covers
// is a Check with StatusUnsupported, not an error.
func (c *Client) VerifyTaxID(ctx context.Context, tid *tax.Identity) (*Check, error) {
	if tid == nil {
		return nil, ErrInput.WithMessage("no tax identity provided")
	}
	if tid.Code == "" {
		return nil, ErrInput.WithMessage("no tax identity code provided")
	}
	id := fromTaxID(tid)
	check := c.check(ctx, id)
	check.Mismatches = mismatches(nil, nil, id, check)
	return check, nil
}

// VerifyIdentity checks one identity. An identity that no verifier covers is
// a Check with StatusUnsupported, not an error.
func (c *Client) VerifyIdentity(ctx context.Context, oid *org.Identity) (*Check, error) {
	if oid == nil {
		return nil, ErrInput.WithMessage("no identity provided")
	}
	if oid.Code == "" {
		return nil, ErrInput.WithMessage("no identity code provided")
	}
	id := fromIdentity(0, oid)
	check := c.check(ctx, id)
	check.Mismatches = mismatches(nil, nil, id, check)
	return check, nil
}

// check runs the first verifier that supports the identifier and shapes the
// outcome into a Check that names the identifier.
func (c *Client) check(ctx context.Context, id identifier) *api.Check {
	v := verifierFor(c.verifiers, id.Identifier)
	if v == nil {
		return id.check(&api.Check{Status: api.StatusUnsupported})
	}
	check, err := v.Verify(ctx, id.Identifier)
	if err != nil {
		check = &api.Check{Source: v.Source()}
		check.Fail(err)
		return id.check(check)
	}
	if check == nil {
		check = &api.Check{Source: v.Source()}
		check.Fail(ErrServer.WithMessage("verifier returned no check"))
	}
	return id.check(check)
}

// check fills the identifier fields of a Check.
func (id identifier) check(c *api.Check) *api.Check {
	c.Path = id.Path
	if id.taxID != nil && c.TaxID == nil {
		c.TaxID = id.taxID
	}
	if id.identity != nil {
		c.Identity = id.identity
	}
	return c
}

// verifierFor returns the first verifier in the set that supports the
// identifier, or nil.
func verifierFor(set []api.Verifier, id api.Identifier) api.Verifier {
	for _, v := range set {
		if v.Supports(id) {
			return v
		}
	}
	return nil
}

// defaultVerifiers is the verifier set that the package-level coverage
// queries consult: the same set New builds without options.
var defaultVerifiers = []api.Verifier{vies.New()}

// SupportsTaxID reports whether the default verifier set covers the tax
// identity. A covered identity can still come back unverified.
func SupportsTaxID(tid *tax.Identity) bool {
	if tid == nil {
		return false
	}
	return verifierFor(defaultVerifiers, fromTaxID(tid).Identifier) != nil
}

// SupportsIdentity reports whether the default verifier set covers the
// identity.
func SupportsIdentity(oid *org.Identity) bool {
	if oid == nil {
		return false
	}
	return verifierFor(defaultVerifiers, fromIdentity(0, oid).Identifier) != nil
}
