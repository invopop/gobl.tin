package vies

import (
	"regexp"
	"slices"
	"strings"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
)

// Italy writes the address in two lines: the street with its house number,
// then the postcode (CAP), the locality and the province code.
//
//	VIA EMILIA EST 1163
//	41122 MODENA MO
var (
	itLocalityLine = regexp.MustCompile(`^(\d{5}) (.+) ([A-Z]{2})$`)
	itHouseNumber  = regexp.MustCompile(`^\d+[A-Z]?(?:[-/]\d+[A-Z]?)?$`)
)

// itStreetExtras are the words that start the part of the street line after
// the house number, such as the staircase or the flat: "INT 3", "SC UN INT 14".
var itStreetExtras = []string{"INT", "INTERNO", "SC", "SCALA", "PIANO", "P", "PAL", "PALAZZINA", "LOTTO", "ED", "EDIFICIO"}

// parseAddressIT parses an Italian address. The province code goes to Region,
// as FatturaPA reads it.
func parseAddressIT(lines []string) *org.Address {
	if len(lines) != 2 {
		return nil
	}
	m := itLocalityLine.FindStringSubmatch(lines[1])
	if m == nil {
		return nil
	}
	out := &org.Address{
		Code:     cbc.Code(m[1]),
		Locality: m[2],
		Region:   m[3],
		Country:  l10n.ISOCountryCode("IT"),
	}
	out.Street, out.Number, out.StreetExtra = splitStreetIT(lines[0])
	return out
}

// splitStreetIT splits the street line at the house number: the first
// number that ends the line or that only an extra such as "INT 3" follows. A
// number inside the street name, as in "VIA 4 NOVEMBRE 10", is part of the
// name. "N" before the number is dropped. A line without such a number is all
// street.
func splitStreetIT(line string) (street, number, extra string) {
	words := strings.Fields(line)
	for i := 1; i < len(words); i++ {
		if !itHouseNumber.MatchString(words[i]) {
			continue
		}
		rest := words[i+1:]
		if len(rest) > 0 && !slices.Contains(itStreetExtras, rest[0]) {
			continue
		}
		name := words[:i]
		if last := name[len(name)-1]; len(name) > 1 && (last == "N" || last == "N.") {
			name = name[:len(name)-1]
		}
		return strings.Join(name, " "), words[i], strings.Join(rest, " ")
	}
	return line, "", ""
}
