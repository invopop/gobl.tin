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
// the user supplied.

// ApplyOptions controls how much of a lookup result is written back into a
// party. The zero value fills an empty name and reports disagreements without
// changing anything the user supplied.
type ApplyOptions struct {
	// CorrectName overwrites a non-empty party name that disagrees with the
	// registry. Off by default: registries often store names upper-cased and
	// with the full legal form, so correcting unconditionally would rewrite
	// every customer's name on every invoice.
	CorrectName bool

	// Address application is deliberately absent. VIES returns the address as
	// a single unstructured string, while GOBL org.Address is structured, so
	// applying it would mean guessing at a parse. Registries that return
	// structured addresses can support an address policy in a later version.
}

// Changes reports what applying a result to a party actually altered.
type Changes struct {
	// Name reports that the party's name was written.
	Name bool

	// NameMismatch reports that the party's name disagrees with the registry,
	// whether or not it was corrected.
	NameMismatch bool
}

// Any reports whether anything changed, and therefore whether the document
// needs writing back. It must OR every write-reporting field of Changes, so
// extend it whenever Changes gains one; a field it misses makes callers skip
// a write-back that did happen.
func (c Changes) Any() bool {
	return c.Name
}

// ApplyTo writes the registry's details into the party, reporting what
// changed. The result must be valid; applying an invalid result is an input
// error. A masked name applies nothing and reports no mismatch.
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
	if r.Name == "" {
		// The registry masked the name, so there is nothing to compare or fill.
		return out, nil
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

	return out, nil
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
