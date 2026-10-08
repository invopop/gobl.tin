// Package tin verifies the identifiers of GOBL parties against the registers
// that issue them, such as VIES for EU VAT numbers, and reports what the
// register holds so that a party can be corrected with a patch.
//
// ABOUT: A Client holds an ordered list of verifiers. For the tax identity of
// a party, the first verifier that supports it answers. When that verifier
// cannot answer, the next verifier that supports the identity is a
// fallback: it can confirm the identity, but it cannot reject it, because a
// second register may simply not hold a valid identity. The Client is safe
// for concurrent use when its verifiers are; it holds no state of its own
// between calls.
//
// Every path in a report is an RFC 6901 JSON Pointer relative to the party
// document, for example "/tax_id" or "/name". A consumer that patches an
// invoice prefixes the party's location, "/customer" or "/supplier". Go
// consumers match on the typed values, Check.TaxID and Mismatch.Field,
// rather than parsing paths.
package tin

import (
	"context"
	"reflect"
	"time"

	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// Client verifies the identifiers of a party with an ordered set of
// verifiers.
//
// The Client holds no cache, so every call reaches the register. Caching, and
// how long an answer stays fresh, is the consuming application's decision.
type Client struct {
	verifiers []Verifier
}

// New creates a Client over the verifiers, in routing order: the first
// verifier that supports an identifier answers for it, and the next ones
// that support it are fallbacks when it cannot answer. A Client with no
// verifiers marks every identifier unsupported. Nil entries are skipped,
// including a typed nil such as a nil *vies.Verifier. Two verifiers may
// share a Source; order still decides which one answers.
func New(verifiers ...Verifier) *Client {
	c := new(Client)
	for _, v := range verifiers {
		if v != nil && !nilValue(v) {
			c.verifiers = append(c.verifiers, v)
		}
	}
	return c
}

// nilValue reports whether the interface wraps a nil pointer, so that a
// typed-nil verifier is skipped like a nil one instead of panicking on its
// first use.
func nilValue(v Verifier) bool {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}

// Verify checks the identifiers of the party: its tax identity. The report
// carries one Check per identifier, and none for a party without a tax ID
// code. A register that cannot answer marks its Check unverified; Verify
// itself returns an error only for a nil party.
func (c *Client) Verify(ctx context.Context, party *org.Party) (*Report, error) {
	if party == nil {
		return nil, ErrInput.WithMessage("no party provided")
	}
	report := &Report{Checks: []*Check{}}
	for _, id := range walk(party) {
		check := c.check(ctx, id, party)
		check.Mismatches = mismatches(party, id, check)
		report.Checks = append(report.Checks, check)
	}
	return report, nil
}

// VerifyTaxID checks one tax identity. An identity that no verifier covers
// is a Check with StatusUnsupported, not an error. The Check has no Path:
// there is no party document to point into.
func (c *Client) VerifyTaxID(ctx context.Context, tid *tax.Identity) (*Check, error) {
	if tid == nil {
		return nil, ErrInput.WithMessage("no tax identity provided")
	}
	id := fromTaxID(tid)
	if id.Code == "" {
		return nil, ErrInput.WithMessage("no tax identity code provided")
	}
	id.Path = ""
	check := c.check(ctx, id, nil)
	check.Mismatches = mismatches(nil, id, check)
	return check, nil
}

// SupportsTaxID reports whether a verifier of the Client covers the tax
// identity. A covered identity can still come back unverified.
func (c *Client) SupportsTaxID(tid *tax.Identity) bool {
	if tid == nil {
		return false
	}
	return c.verifierFor(fromTaxID(tid).Identifier) != nil
}

// check asks the first verifier that supports the identifier and builds
// the Check from its answer. When that verifier cannot answer, the next
// verifiers that support the identifier are asked in order, and the first
// valid answer wins. A fallback's invalid answer is not used: the Check stays
// unverified with the first verifier's failure. The party is nil for a
// single identifier.
func (c *Client) check(ctx context.Context, id identifier, party *org.Party) *Check {
	check := &Check{Path: id.Path, TaxID: id.taxID}
	req := Request{Identifier: id.Identifier, Party: party}
	verifiers := c.verifiersFor(id.Identifier)
	if len(verifiers) == 0 {
		check.Status = StatusUnsupported
		return check
	}
	for i, v := range verifiers {
		if i > 0 && ctx.Err() != nil {
			break
		}
		ans, err := ask(ctx, v, req)
		if err != nil {
			if i == 0 {
				check.Source = v.Source()
				check.fail(err)
			}
			continue
		}
		if i > 0 && ans.Status != StatusValid {
			continue
		}
		check.Source = v.Source()
		check.fail(nil)
		check.Status = ans.Status
		check.Record = ans.Record
		check.CheckedAt = ans.CheckedAt
		if check.CheckedAt.IsZero() {
			check.CheckedAt = time.Now().UTC()
		}
		if ans.TaxID != nil {
			check.TaxID = normalizeTaxID(ans.TaxID)
		}
		return check
	}
	return check
}

// ask runs one verifier and returns its answer when it is valid or invalid.
// Any other answer is a server error.
func ask(ctx context.Context, v Verifier, req Request) (*Answer, error) {
	ans, err := v.Verify(ctx, req)
	switch {
	case err != nil:
		return nil, err
	case ans == nil:
		return nil, ErrServer.WithMessage("verifier returned no answer")
	case ans.Status != StatusValid && ans.Status != StatusInvalid:
		return nil, ErrServer.WithMsgf("verifier returned status %q", ans.Status)
	default:
		return ans, nil
	}
}

// verifiersFor returns the verifiers that support the identifier, in
// routing order.
func (c *Client) verifiersFor(id Identifier) []Verifier {
	var out []Verifier
	for _, v := range c.verifiers {
		if v.Supports(id) {
			out = append(out, v)
		}
	}
	return out
}

// verifierFor returns the first verifier that supports the identifier, or
// nil.
func (c *Client) verifierFor(id Identifier) Verifier {
	for _, v := range c.verifiers {
		if v.Supports(id) {
			return v
		}
	}
	return nil
}
