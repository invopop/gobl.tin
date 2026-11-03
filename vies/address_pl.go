package vies

import (
	"regexp"
	"strings"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
)

// Two lines: "JESIONOWA 13 M5" and "80-261 GDAŃSK". A third line before them
// names the town or village when it differs from the post town, or when the
// post town is absent: "NIEKANIN" / "SZMARAGDOWA 3" / "78-100 KOŁOBRZEG".
var (
	plStreetLine   = regexp.MustCompile(`^(.+?) (\d+[A-Z]?(?:[/-]\d+[A-Z]?)?)(?: (M[\dA-Z.]+))?$`)
	plPostcodeLine = regexp.MustCompile(`^(\d{2}-\d{3})(?: (.+))?$`)
	plDigit        = regexp.MustCompile(`\d`)
	// A number inside the street name, as in "SUCHA 1 LOKAL 5", hides the
	// house number. A name may still start with one ("3 MAJA") or follow an
	// abbreviation ("AL. 3 MAJA").
	plNumberInName = regexp.MustCompile(`[^ .] \d`)
)

// The flat ("M5") stays with its label in StreetExtra. A village with a
// separate post town goes to StreetExtra and the post town to Locality, as
// Polish post writes it.
func parseAddressPL(lines []string) *org.Address {
	var place string
	switch len(lines) {
	case 2:
	case 3:
		place = lines[0]
		if plDigit.MatchString(place) {
			return nil
		}
	default:
		return nil
	}
	s := plStreetLine.FindStringSubmatch(lines[len(lines)-2])
	p := plPostcodeLine.FindStringSubmatch(lines[len(lines)-1])
	if s == nil || p == nil || plNumberInName.MatchString(s[1]) {
		return nil
	}
	locality := p[2]
	var extra []string
	if s[3] != "" {
		extra = append(extra, s[3])
	}
	switch {
	case locality == "" && place == "":
		return nil
	case locality == "":
		locality = place
	case place != "" && place != locality:
		extra = append(extra, place)
	}
	return &org.Address{
		Street:      s[1],
		Number:      s[2],
		StreetExtra: strings.Join(extra, ", "),
		Code:        cbc.Code(p[1]),
		Locality:    locality,
		Country:     l10n.ISOCountryCode("PL"),
	}
}
