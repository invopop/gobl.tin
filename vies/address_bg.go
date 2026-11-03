package vies

import (
	"regexp"
	"strings"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
)

// One line: "ул. Гюешево №14 обл.СОФИЯ, гр.СОФИЯ 1303". The street part
// labels each detail: "бл.86 вх.А ет.1 ап.2".
var (
	bgTail = regexp.MustCompile(`^(.*?) ?обл\. ?([^,]+?),? (?:гр|с|ГР|С)\. ?(.+?) (\d{4})$`)
	// A label starts the street part or follows a space or a comma.
	bgLabelStart = regexp.MustCompile(`(?:^|[ ,])(?:№|No\.? ?\d|бл\.|вх\.|ет\.|ап\.|офис|магазин)`)
	bgNumber     = regexp.MustCompile(`^(?:№|No\.?) ?(\d+[А-Яа-я]?(?:-\d+[А-Яа-я]?|-[А-Я])?)( [А-Я])?`)
	bgDetail     = regexp.MustCompile(`^(бл\.|вх\.|ет\.|ап\.) ?(офис ?[^ ,]+|[^ ,]+)`)
	bgPremises   = regexp.MustCompile(`^(офис|магазин) ?(?:№ ?)?([^ ,]+)`)
	// A housing estate or a neighbourhood, then a street, sometimes glued:
	// "жк Дианабадул. Ловен парк".
	bgEstateStreet = regexp.MustCompile(`^((?:жк|кв\.) ?.+?) ?((?:ул|бул)\. ?.+)$`)
	bgBareNumber   = regexp.MustCompile(`^((?i:ул|бул|пл)\. ?.+) (\d+[А-Яа-я]?|\d+-[А-Я])$`)
	// Two words without a space: "БАДЕМАкомплекс", "местностГрахорлива".
	bgGlued = regexp.MustCompile(`[а-я][А-Я]|[А-Я]{2}[а-я]`)
)

// The province goes to Region. The labels of the block, entrance, floor, flat
// and office stay in StreetExtra, as the register writes them.
func parseAddressBG(lines []string) *org.Address {
	if len(lines) != 1 {
		return nil
	}
	m := bgTail.FindStringSubmatch(lines[0])
	if m == nil || m[1] == "" {
		return nil
	}
	name, rest := m[1], ""
	if loc := bgLabelStart.FindStringIndex(m[1]); loc != nil {
		name, rest = strings.TrimRight(m[1][:loc[0]], " ,"), m[1][loc[0]:]
	}
	num, extra, ok := splitDetailsBG(rest)
	if !ok {
		return nil
	}
	street, estate, ok := splitNameBG(name)
	if !ok {
		return nil
	}
	if num == "" {
		if b := bgBareNumber.FindStringSubmatch(street); b != nil {
			street, num = b[1], b[2]
		}
	}
	if estate != "" {
		extra = append([]string{estate}, extra...)
	}
	return &org.Address{
		Street:      street,
		Number:      num,
		StreetExtra: strings.Join(extra, " "),
		Code:        cbc.Code(m[4]),
		Locality:    m[3],
		Region:      m[2],
		Country:     l10n.ISOCountryCode("BG"),
	}
}

// Every word after the first label must belong to a label.
func splitDetailsBG(rest string) (num string, extra []string, ok bool) {
	for rest = strings.TrimLeft(rest, " ,"); rest != ""; rest = strings.TrimLeft(rest, " ,") {
		if m := bgNumber.FindStringSubmatch(rest); m != nil && num == "" {
			num, rest = m[1], rest[len(m[0]):]
			if m[2] != "" {
				if rest != "" && rest[0] != ' ' && rest[0] != ',' {
					return "", nil, false
				}
				num += m[2]
			}
		} else if m := bgDetail.FindStringSubmatch(rest); m != nil {
			extra, rest = append(extra, m[1]+m[2]), rest[len(m[0]):]
		} else if m := bgPremises.FindStringSubmatch(rest); m != nil {
			extra, rest = append(extra, m[1]+" "+m[2]), rest[len(m[0]):]
		} else {
			return "", nil, false
		}
	}
	return num, extra, true
}

// A name with a comma, a number sign or two glued words has no clear street.
func splitNameBG(name string) (street, estate string, ok bool) {
	if name == "" || strings.ContainsAny(name, ",№") {
		return "", "", false
	}
	street = name
	if m := bgEstateStreet.FindStringSubmatch(name); m != nil {
		estate, street = m[1], m[2]
	}
	if bgGlued.MatchString(street) || bgGlued.MatchString(estate) {
		return "", "", false
	}
	return street, estate, true
}
