package vies

import (
	"regexp"
	"strings"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
)

// Two lines: "BLANKWATER 00012" and "6071NV SWALMEN". The house number is
// padded to five digits.
var (
	nlStreetLine   = regexp.MustCompile(`^(.+?) (\d{5})(?: (.+))?$`)
	nlPostcodeLine = regexp.MustCompile(`^([1-9]\d{3}[A-Z]{2}) (.+)$`)
	// A house number addition: "C", "A-1", "BS", "14".
	nlAddition = regexp.MustCompile(`^[A-Z\d]{1,4}(?:-[A-Z\d]{1,4})?$`)
)

// A shielded address names the trade register rule instead of a place.
const nlShielded = "ADRES AFGESCHERMD"

// A PO box has no street.
const nlPOBox = "POSTBUS "

// The padding zeros are dropped from the house number. A short addition
// stays with the number; longer details, such as "UNIT C.2.17", go to
// StreetExtra.
func parseAddressNL(lines []string) *org.Address {
	if len(lines) != 2 || strings.HasPrefix(lines[0], nlShielded) || strings.HasPrefix(lines[0], nlPOBox) {
		return nil
	}
	s := nlStreetLine.FindStringSubmatch(lines[0])
	p := nlPostcodeLine.FindStringSubmatch(lines[1])
	if s == nil || p == nil {
		return nil
	}
	num := strings.TrimLeft(s[2], "0")
	if num == "" {
		return nil
	}
	out := &org.Address{
		Street:   s[1],
		Number:   num,
		Code:     cbc.Code(p[1]),
		Locality: p[2],
		Country:  l10n.ISOCountryCode("NL"),
	}
	if s[3] != "" {
		if nlAddition.MatchString(s[3]) {
			out.Number += " " + s[3]
		} else {
			out.StreetExtra = s[3]
		}
	}
	return out
}
