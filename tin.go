// Package tin verifies the identifiers of GOBL parties against the registers
// that issue them, such as VIES for EU VAT numbers, and reports what the
// register holds so that a party can be corrected with a patch.
//
// ABOUT: A Client holds an ordered list of verifiers. For each identifier of
// a party, the tax identity first and then each identity in order, the first
// verifier that supports it answers. The Client is safe for concurrent use
// when its verifiers are; it holds no state of its own between calls.
//
// Every path in a report is an RFC 6901 JSON Pointer relative to the party
// document, for example "/tax_id" or "/identities/1/code". A consumer that
// patches an invoice prefixes the party's location, "/customer" or
// "/supplier". Go consumers match on the typed values, Check.TaxID,
// Check.Identity and Mismatch.Field, rather than parsing paths.
package tin

import (
	"context"

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
// verifier that supports an identifier answers for it. A Client with no
// verifiers marks every identifier unsupported. Nil entries are skipped. Two
// verifiers may share a Source; order still decides which one answers.
func New(verifiers ...Verifier) *Client {
	c := new(Client)
	for _, v := range verifiers {
		if v != nil {
			c.verifiers = append(c.verifiers, v)
		}
	}
	return c
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
	report := NewVerification(party)
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

// SupportsTaxID reports whether a verifier of the Client covers the tax
// identity. A covered identity can still come back unverified.
func (c *Client) SupportsTaxID(tid *tax.Identity) bool {
	if tid == nil {
		return false
	}
	return c.verifierFor(fromTaxID(tid).Identifier) != nil
}

// SupportsIdentity reports whether a verifier of the Client covers the
// identity.
func (c *Client) SupportsIdentity(oid *org.Identity) bool {
	if oid == nil {
		return false
	}
	return c.verifierFor(fromIdentity(0, oid).Identifier) != nil
}

// check runs the first verifier that supports the identifier and shapes the
// outcome into a Check that names the identifier.
func (c *Client) check(ctx context.Context, id identifier) *Check {
	v := c.verifierFor(id.Identifier)
	if v == nil {
		return id.check(&Check{Status: StatusUnsupported})
	}
	check, err := v.Verify(ctx, id.Identifier)
	if err != nil {
		check = &Check{Source: v.Source()}
		check.Fail(err)
		return id.check(check)
	}
	if check == nil {
		check = &Check{Source: v.Source()}
		check.Fail(ErrServer.WithMessage("verifier returned no check"))
	}
	return id.check(check)
}

// check fills the identifier fields of a Check.
func (id identifier) check(c *Check) *Check {
	c.Path = id.Path
	if id.taxID != nil && c.TaxID == nil {
		c.TaxID = id.taxID
	}
	if id.identity != nil {
		c.Identity = id.identity
	}
	return c
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
