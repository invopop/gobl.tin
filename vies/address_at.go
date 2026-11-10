package vies

import (
	"regexp"
	"strings"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
)

// Two lines: "Argentinierstraße 18/1/10" and "AT-1040 Wien". Stair, door and
// floor follow the house number after slashes: "2/Stiege B6/OG".
var (
	atStreetLine   = regexp.MustCompile(`^(\D+?) (\d+(?: ?[A-Za-z])?(?: ?- ?\d+(?: ?[A-Za-z])?)?(?:/.+)?)$`)
	atPostcodeLine = regexp.MustCompile(`^AT-([1-9]\d{3}) (.+)$`)
)

// The slash parts stay in Number, as Austrian addresses write them. A digit in
// the street name, as in "IZ NÖ-Süd Str 3 Obj 16", hides the house number.
func parseAddressAT(lines []string) *org.Address {
	if len(lines) != 2 || strings.Contains(lines[0], ",") {
		return nil
	}
	s := atStreetLine.FindStringSubmatch(lines[0])
	p := atPostcodeLine.FindStringSubmatch(lines[1])
	if s == nil || p == nil {
		return nil
	}
	return &org.Address{
		Street:   s[1],
		Number:   s[2],
		Code:     cbc.Code(p[1]),
		Locality: p[2],
		Country:  l10n.ISOCountryCode("AT"),
	}
}
