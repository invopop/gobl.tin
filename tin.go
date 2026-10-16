// Package tin verifies the identifiers of GOBL parties against the registers
// that issue them.
package tin

import (
	"context"
	"reflect"
	"time"

	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// Client verifies the identifiers of a party with an ordered list of
// verifiers. It holds no cache.
type Client struct {
	verifiers []Verifier
}

// New creates a Client over the verifiers, in routing order. Nil verifiers
// are skipped.
func New(verifiers ...Verifier) *Client {
	c := new(Client)
	for _, v := range verifiers {
		if v != nil && !nilValue(v) {
			c.verifiers = append(c.verifiers, v)
		}
	}
	return c
}

// Catches a typed nil, such as a nil *vies.Verifier.
func nilValue(v Verifier) bool {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}

// Verify checks the party's tax ID. It returns an error only for a nil party.
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

// VerifyTaxID checks one tax identity. The Check has no Path.
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

// SupportsTaxID reports whether a verifier covers the tax identity.
func (c *Client) SupportsTaxID(tid *tax.Identity) bool {
	if tid == nil {
		return false
	}
	return c.verifierFor(fromTaxID(tid).Identifier) != nil
}

// Only the first verifier that supports the identifier is asked; when it
// cannot answer, the Check is unverified.
func (c *Client) check(ctx context.Context, id identifier, party *org.Party) *Check {
	check := &Check{Path: id.Path, TaxID: id.taxID}
	v := c.verifierFor(id.Identifier)
	if v == nil {
		check.Status = StatusUnsupported
		return check
	}
	check.Source = v.Source()
	ans, err := ask(ctx, v, Request{Identifier: id.Identifier, Party: party})
	if err != nil {
		check.fail(err)
		return check
	}
	check.Status = ans.Status
	check.Record = ans.Record
	check.Reason = ans.Reason
	check.CheckedAt = ans.CheckedAt
	if check.CheckedAt.IsZero() {
		check.CheckedAt = time.Now().UTC()
	}
	if ans.TaxID != nil {
		check.TaxID = normalizeTaxID(ans.TaxID)
	}
	return check
}

// Any answer other than valid or invalid is a server error.
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

func (c *Client) verifierFor(id Identifier) Verifier {
	for _, v := range c.verifiers {
		if v.Supports(id) {
			return v
		}
	}
	return nil
}
