package vies

import (
	"strings"

	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
)

// Address parsers by member state; each member state writes its own way.
var addressParsers = map[l10n.TaxCountryCode]func(lines []string) *org.Address{
	"IT": parseAddressIT,
}

// Nil unless a parser recognises the whole text: a wrong address is worse than
// none.
func parseAddress(country l10n.TaxCountryCode, raw string) *org.Address {
	parse := addressParsers[country]
	if parse == nil {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(unmask(raw), "\n") {
		if l = strings.Join(strings.Fields(l), " "); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return parse(lines)
}
