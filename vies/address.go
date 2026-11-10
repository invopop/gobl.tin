package vies

import (
	"strings"

	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
)

// Address parsers by member state; each member state writes its own way. A
// parser gets the text as VIES gives it.
var addressParsers = map[l10n.TaxCountryCode]func(raw string) *org.Address{
	"AT": byLines(parseAddressAT),
	"BG": byLines(parseAddressBG),
	"IT": byLines(parseAddressIT),
	"NL": byLines(parseAddressNL),
	"PL": byLines(parseAddressPL),
}

// Nil unless a parser recognises the whole text: a wrong address is worse than
// none.
func parseAddress(country l10n.TaxCountryCode, raw string) *org.Address {
	parse := addressParsers[country]
	if parse == nil || unmask(raw) == "" {
		return nil
	}
	return parse(raw)
}

// byLines gives a parser the lines of the text, with the spaces normalised and
// without empty lines.
func byLines(parse func(lines []string) *org.Address) func(raw string) *org.Address {
	return func(raw string) *org.Address {
		lines := addressLines(raw)
		if len(lines) == 0 {
			return nil
		}
		return parse(lines)
	}
}

func addressLines(raw string) []string {
	var lines []string
	for _, l := range strings.Split(raw, "\n") {
		if l = strings.Join(strings.Fields(l), " "); l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
