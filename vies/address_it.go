package vies

import (
	"regexp"
	"slices"
	"strings"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
)

// Two lines: "VIA EMILIA EST 1163" and "41122 MODENA MO".
var (
	itLocalityLine = regexp.MustCompile(`^(\d{5}) (.+) ([A-Z]{2})$`)
	itHouseNumber  = regexp.MustCompile(`^\d+[A-Z]?(?:[-/]\d+[A-Z]?)?$`)
)

// Words that may follow the house number, such as "INT 3" or "SC UN INT 14".
var itStreetExtras = []string{"INT", "INTERNO", "SC", "SCALA", "PIANO", "P", "PAL", "PALAZZINA", "LOTTO", "ED", "EDIFICIO"}

// Street types a street line must start with, so that a line such as
// "C/O STUDIO ROSSI" is never taken for a street.
var itStreetTypes = []string{
	"VIA", "VIALE", "VIALETTO", "PIAZZA", "PIAZZALE", "PIAZZETTA", "CORSO", "LARGO",
	"VICOLO", "VICO", "STRADA", "STRADONE", "STRADELLA", "CONTRADA", "LOCALITA", "LOC",
	"FRAZIONE", "FRAZ", "BORGO", "BORGATA", "LUNGOTEVERE", "LUNGOMARE", "LUNGARNO",
	"LUNGOLAGO", "SALITA", "CALLE", "CAMPO", "CAMPIELLO", "FONDAMENTA", "RIONE", "CORTE",
	"TRAVERSA", "GALLERIA", "PASSAGGIO", "ZONA", "REGIONE", "CASCINA", "CIRCONVALLAZIONE",
	"RAMPA", "SCALINATA", "ALZAIA", "BANCHINA", "MOLO", "PARCO", "QUARTIERE", "SS", "SP",
}

// The province code goes to Region, as FatturaPA reads it.
func parseAddressIT(lines []string) *org.Address {
	if len(lines) != 2 || !isStreetIT(lines[0]) {
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

// The house number is the first number that ends the line or that only an
// extra follows, so "VIA 4 NOVEMBRE 10" keeps 4 in the name. "N" before it is
// dropped.
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

func isStreetIT(line string) bool {
	first, _, _ := strings.Cut(line, " ")
	return slices.Contains(itStreetTypes, strings.TrimSuffix(first, "."))
}
