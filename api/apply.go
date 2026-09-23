package api

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/invopop/gobl/org"
)

// ABOUT: Mapping a lookup result onto a GOBL party. Everything here is
// deliberately conservative: this code writes into documents that consumers
// send to their own customers, so it fills gaps rather than rewriting what
// the user supplied. Conflicting identifiers are never overwritten, only
// reported.

// AddressPolicy controls whether a result's registered address is written
// into the party.
type AddressPolicy string

// Address policies.
const (
	// AddressPolicyNone leaves the party's addresses untouched. It is the
	// default: an invoice address is usually the trading address, not the
	// registered office.
	AddressPolicyNone AddressPolicy = ""

	// AddressPolicyAppend adds the registered address alongside any the
	// party already has, at most once.
	AddressPolicyAppend AddressPolicy = "append"

	// AddressPolicyReplace makes the registered address the party's first
	// address.
	AddressPolicyReplace AddressPolicy = "replace"
)

// ApplyOptions controls how much of a lookup result is written back into a
// party. The zero value fills gaps and reports disagreements without
// changing anything the user supplied.
type ApplyOptions struct {
	// CorrectName overwrites a non-empty party name that disagrees with the
	// registry. Off by default: registries often store names upper-cased and
	// with the full legal form, so correcting unconditionally would rewrite
	// every customer's name on every invoice.
	CorrectName bool

	// Address decides whether a structured registered address is written in.
	Address AddressPolicy
}

// Changes reports what applying a result to a party actually altered.
type Changes struct {
	// Name reports that the party's name was written.
	Name bool

	// TaxID reports that the party's tax identity was written.
	TaxID bool

	// Identities reports that the party's identities were written.
	Identities bool

	// Addresses reports that the party's addresses were written.
	Addresses bool

	// NameMismatch reports that the party's name disagrees with the registry,
	// whether or not it was corrected.
	NameMismatch bool

	// TaxIDConflict reports that the party asserts a different tax identity,
	// which is never overwritten.
	TaxIDConflict bool

	// IdentityConflict reports that the party carries an identity of the same
	// type as the registry's with a different code, which is never
	// overwritten.
	IdentityConflict bool
}

// Any reports whether anything changed, and therefore whether the document
// needs writing back. Extend it whenever Changes gains a write-reporting
// field.
func (c Changes) Any() bool {
	return c.Name || c.TaxID || c.Identities || c.Addresses
}

// ApplyTo writes the registry's details into the party, reporting what
// changed. The result must be valid; applying an invalid result is an input
// error. Masked or absent fields apply nothing.
func (r *Result) ApplyTo(party *org.Party, opts ApplyOptions) (Changes, error) {
	var out Changes
	if r == nil {
		return out, ErrInput.WithMessage("no result provided")
	}
	if party == nil {
		return out, ErrInput.WithMessage("no party provided")
	}
	if !r.Valid {
		return out, ErrInput.WithMessage("cannot apply an invalid result")
	}

	r.applyName(party, opts, &out)
	r.applyTaxID(party, &out)
	r.applyIdentities(party, &out)
	r.applyAddress(party, opts.Address, &out)

	return out, nil
}

// applyName fills or corrects the party's name under the naming policy.
func (r *Result) applyName(party *org.Party, opts ApplyOptions, out *Changes) {
	if r.Name == "" {
		// The registry masked the name, so there is nothing to compare or
		// fill.
		return
	}
	switch {
	case party.Name == "":
		party.Name = r.Name
		out.Name = true
	case NameMatches(r.Name, party.Name):
		// The party's name agrees with the registry; keep the user's casing.
	case opts.CorrectName:
		party.Name = r.Name
		out.Name, out.NameMismatch = true, true
	default:
		out.NameMismatch = true
	}
}

// applyTaxID fills an absent tax identity and reports, never resolves, a
// disagreement with a present one.
func (r *Result) applyTaxID(party *org.Party, out *Changes) {
	if r.TaxID == nil {
		return
	}
	if party.TaxID == nil {
		tid := *r.TaxID
		party.TaxID = &tid
		out.TaxID = true
		return
	}
	if party.TaxID.Country != r.TaxID.Country || party.TaxID.Code != r.TaxID.Code {
		out.TaxIDConflict = true
	}
}

// applyIdentities appends registry identities the party does not carry. An
// identity of the same type with a different code is a conflict and is left
// alone. Appending keeps re-runs idempotent.
func (r *Result) applyIdentities(party *org.Party, out *Changes) {
	for _, id := range r.Identities {
		if id == nil {
			continue
		}
		conflict := false
		present := false
		for _, existing := range party.Identities {
			if existing == nil || existing.Type != id.Type || existing.Key != id.Key {
				continue
			}
			if existing.Code == id.Code {
				present = true
			} else {
				conflict = true
			}
			break
		}
		switch {
		case present:
		case conflict:
			out.IdentityConflict = true
		default:
			cp := *id
			party.Identities = append(party.Identities, &cp)
			out.Identities = true
		}
	}
}

// applyAddress writes the registered address into the party according to the
// policy. A result without a structured address applies nothing.
func (r *Result) applyAddress(party *org.Party, policy AddressPolicy, out *Changes) {
	if policy == AddressPolicyNone || r.Address == nil {
		return
	}
	addr := *r.Address

	switch {
	case len(party.Addresses) == 0:
		party.Addresses = []*org.Address{&addr}
		out.Addresses = true
	case policy == AddressPolicyReplace:
		party.Addresses[0] = &addr
		out.Addresses = true
	default:
		// The party already has an address, probably its billing or trading
		// one. Add the registered address alongside, but only once.
		for _, existing := range party.Addresses {
			if existing != nil && existing.Label == addr.Label {
				return
			}
		}
		party.Addresses = append(party.Addresses, &addr)
		out.Addresses = true
	}
}

// nameElide is punctuation that joins rather than separates, so that
// "A.C.M.E." and "O'Brien" reduce to "ACME" and "OBRIEN" rather than
// splitting into pieces.
var nameElide = regexp.MustCompile(`[.'’]+`)

// NameMatches reports whether two names agree once case, punctuation and
// surrounding whitespace are folded. It compares; it never rewrites.
func NameMatches(a, b string) bool {
	na := normalizeName(a)
	return na != "" && na == normalizeName(b)
}

// normalizeName reduces a name to a comparison key. It keeps letters and
// digits in any script, because registry names across the EU are not ASCII.
func normalizeName(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "&", " AND ")
	s = nameElide.ReplaceAllString(s, "")
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, s)
	return strings.Join(strings.Fields(s), " ")
}
