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

// The first verifier that supports the identifier decides: its valid or
// invalid answer is final. Only when it cannot answer are the next ones
// asked, and they can confirm but never reject, because a second register may
// not hold a valid identifier (VIES lacks most Spanish NIFs). A fallback's
// invalid answer is ignored, and the Check keeps the first verifier's failure.
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

func (c *Client) verifiersFor(id Identifier) []Verifier {
	var out []Verifier
	for _, v := range c.verifiers {
		if v.Supports(id) {
			out = append(out, v)
		}
	}
	return out
}

func (c *Client) verifierFor(id Identifier) Verifier {
	for _, v := range c.verifiers {
		if v.Supports(id) {
			return v
		}
	}
	return nil
}
