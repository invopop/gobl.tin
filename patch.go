package tin

import (
	"encoding/json"
	"strings"
	"unicode"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

// ABOUT: A patch is an RFC 7396 JSON merge patch on the party, with what the
// register holds and the party does not: a best effort for each field the
// register gives in a form GOBL can take as is. The name is written whenever
// the register gives one that is not already the party's, exactly: the
// register's name is the legal one. The address is written only
// when the register gives it structured, and it becomes the party's first
// address while the others stay. The tax ID is never changed: it is what
// was verified.
//
// Checks are read in report order and the first valid record that has a
// value for a field wins. Records behind invalid, unverified or unsupported
// checks are never read.

// Patch builds a JSON merge patch on the party from its report. A nil party
// or report is an input error. A patch with nothing to write is "{}".
func Patch(party *org.Party, report *Report) (json.RawMessage, error) {
	if party == nil {
		return nil, ErrInput.WithMessage("no party provided")
	}
	if report == nil {
		return nil, ErrInput.WithMessage("no report provided")
	}
	patch := map[string]any{}
	if name := patchName(party, report); name != "" {
		patch["name"] = name
	}
	if addrs := patchAddresses(party, report); addrs != nil {
		patch["addresses"] = addrs
	}
	return json.Marshal(patch)
}

// patchName returns the registered name when it is not already the
// party's, or empty. The name is trimmed, and one with no letter or digit,
// such as a "---" placeholder, is no name.
func patchName(party *org.Party, report *Report) string {
	for _, rec := range validRecords(party, report) {
		name := strings.TrimSpace(rec.Name)
		if !hasLetterOrDigit(name) {
			continue
		}
		if name != party.Name {
			return name
		}
		return ""
	}
	return ""
}

// hasLetterOrDigit reports whether s holds at least one letter or digit.
func hasLetterOrDigit(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// patchAddresses returns the full addresses array with the registered
// address first, or nil when the party already has it first. When the party
// holds it further down, it moves to the front and nothing is dropped;
// otherwise it replaces the first address. A merge patch replaces arrays
// whole, so the party's other addresses are carried along.
func patchAddresses(party *org.Party, report *Report) []*org.Address {
	for _, rec := range validRecords(party, report) {
		if rec.Address == nil || postalOf(rec.Address) == (postal{}) {
			// An address with no postal field is no address.
			continue
		}
		reg := postalOf(rec.Address)
		at := -1
		for i, a := range party.Addresses {
			if a != nil && postalOf(a) == reg {
				at = i
				break
			}
		}
		if at == 0 {
			return nil
		}
		out := []*org.Address{rec.Address}
		for i, a := range party.Addresses {
			// A nil entry would become a null in the array. The first
			// address is replaced unless the registered one was found.
			if a == nil || i == at || (at < 0 && i == 0) {
				continue
			}
			out = append(out, a)
		}
		return out
	}
	return nil
}

// validRecords lists the records behind valid checks of the party's own tax
// ID, in report order. A check's tax ID is the one the register echoed, so a
// check for another tax ID, because the register answered for another one or
// the report belongs to another party, describes another party and never
// feeds the patch.
func validRecords(party *org.Party, report *Report) []*Record {
	var out []*Record
	for _, c := range report.Checks {
		if c != nil && c.Status == StatusValid && c.Record != nil && sameTaxID(c.TaxID, party.TaxID) {
			out = append(out, c.Record)
		}
	}
	return out
}

// sameTaxID reports whether two tax identities name the same country and
// code once normalized. A missing identity matches nothing.
func sameTaxID(a, b *tax.Identity) bool {
	if a == nil || b == nil {
		return false
	}
	na, nb := normalizeTaxID(a), normalizeTaxID(b)
	return na.Country == nb.Country && na.Code == nb.Code
}

// postal is the comparable projection of an address: the fields that place
// it, without the label, coordinates or metadata. A label names an address
// but does not locate it.
type postal struct {
	poBox, number, street, streetExtra, locality, region string
	code                                                 cbc.Code
	country                                              l10n.ISOCountryCode
}

// postalOf projects an address onto its postal fields.
func postalOf(a *org.Address) postal {
	return postal{
		poBox:       a.PostOfficeBox,
		number:      a.Number,
		street:      a.Street,
		streetExtra: a.StreetExtra,
		locality:    a.Locality,
		region:      a.Region,
		code:        a.Code,
		country:     a.Country,
	}
}
