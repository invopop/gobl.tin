package vies

import (
	"strings"

	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
)

// addressParsers turn the free-text address that VIES gives for a member
// state into a structured GOBL address. Each member state answers from its
// own register, so each writes the address its own way, and a parser covers
// one member state only. A member state without a parser gives no address.
var addressParsers = map[l10n.TaxCountryCode]func(lines []string) *org.Address{
	"IT": parseAddressIT,
}

// parseAddress returns the structured address for the VIES text of the
// member state, or nil when there is no parser or the text does not have the
// shape the parser knows. A wrong address is worse than none, so a parser
// accepts only the whole shape.
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
