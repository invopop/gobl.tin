package vies

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
)

// Three lines: the street text, the place and the post town, as in
// "RUA CASTILHO N 65 3 DTO" / "LISBOA" / "1250-068 LISBOA". VIES cuts a
// longer street text after 60 characters, also inside a word, and goes on in
// the next line.
const ptCut = 60

var (
	ptPostcodeLine = regexp.MustCompile(`^(\d{4}-\d{3}) (.+)$`)
	// The number marker: "N 65", "Nº 101", "NUMERO 230".
	ptNumbered     = regexp.MustCompile(`^(.+?),? (?:N|N\.|Nº|Nº\.|N\.º|N°|NUMERO|NÚMERO) (\d+[A-Z]?(?:-\d+[A-Z]?)?)(?:,? (.+))?$`)
	ptLetter       = regexp.MustCompile(`^[A-Z]$`)
	ptBrackets     = regexp.MustCompile(`\(.*?\)`)
	ptMunicipality = regexp.MustCompile(`^[A-Z]{3}$`)
)

var ptStreetTypes = []string{
	"R", "RUA", "AV", "AVENIDA", "AL", "ALMD", "ALAMEDA", "LG", "LARGO", "PC", "PRAÇA", "PRACA",
	"PRACETA", "TV", "TRAVESSA", "EST", "ESTR", "ESTRADA", "ROT", "ROTUNDA", "CAMINHO", "CALÇADA",
	"CALCADA", "BECO", "BAIRRO", "QUINTA", "LUGAR", "ZONA", "URBANIZAÇÃO", "PARQUE", "CAMPO", "VIA",
}

// Before the marker, these words make it the number of a lot, a building or
// a room: "ZONA INDUSTRIAL DE VAGOS LOTE N 61".
var ptOtherNumbers = []string{
	"LOTE", "LT", "EDIFICIO", "EDIFÍCIO", "EDF", "BLOCO", "PISO", "ANDAR", "SALA", "LOJA", "LJ",
	"ARMAZEM", "ARMAZÉM", "FRAÇÃO", "FRACÇÃO", "FRACAO", "ESCRITORIO", "ESCRITÓRIO", "ESC",
	"NUCLEO", "NÚCLEO", "MODULO", "MÓDULO", "PORTA", "KM",
}

// A road before the marker makes it the road number: "ESTRADA NACIONAL N 8".
var ptRoads = []string{"NACIONAL", "MUNICIPAL", "REGIONAL", "EN", "EM", "ER"}

// The house number is read only after a marker that follows a street type.
// Any other street text goes whole to Street. The place goes to StreetExtra
// when it is not the post town, as CTT writes it.
func parseAddressPT(raw string) *org.Address {
	var lines []string
	for _, l := range strings.Split(raw, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	switch {
	case len(lines) == 4 && utf8.RuneCountInString(lines[0]) == ptCut:
		// The space at a cut between two words stays at one end of a line.
		lines = append([]string{lines[0] + lines[1]}, lines[2:]...)
	case len(lines) != 3:
		return nil
	}
	for i, l := range lines {
		lines[i] = strings.Join(strings.Fields(l), " ")
	}
	street, place := lines[0], lines[1]
	p := ptPostcodeLine.FindStringSubmatch(lines[2])
	if p == nil || isPOBoxPT(street) {
		return nil
	}
	out := &org.Address{
		Street:   street,
		Code:     cbc.Code(p[1]),
		Locality: p[2],
		Country:  l10n.ISOCountryCode("PT"),
	}
	var extra []string
	if n := ptNumbered.FindStringSubmatch(street); n != nil && isHouseNumberPT(n[1]) {
		out.Street, out.Number = strings.TrimRight(n[1], " -,"), n[2]
		if rest := strings.TrimLeft(n[3], " -,"); ptLetter.MatchString(rest) {
			out.Number += " " + rest
		} else if rest != "" {
			extra = append(extra, rest)
		}
	}
	if !samePlacePT(place, out.Locality) {
		extra = append(extra, place)
	}
	out.StreetExtra = strings.Join(extra, ", ")
	return out
}

func isPOBoxPT(street string) bool {
	return slices.Contains(strings.Fields(street), "APARTADO") || strings.Contains(street, "CAIXA POSTAL")
}

func isHouseNumberPT(name string) bool {
	words := strings.Fields(name)
	if len(words) == 0 || !slices.Contains(ptStreetTypes, words[0]) {
		return false
	}
	// "LT." is LT.
	for i, w := range words {
		words[i] = strings.TrimFunc(w, unicode.IsPunct)
	}
	if slices.Contains(ptRoads, words[len(words)-1]) {
		return false
	}
	return !slices.ContainsFunc(words, func(w string) bool { return slices.Contains(ptOtherNumbers, w) })
}

// Accents, hyphens, brackets and a municipality suffix ("AMT") are ignored:
// "PONTA DELGADA (AÇORES)" is the post town "PONTA DELGADA".
func samePlacePT(place, town string) bool {
	a := tin.NameKey(ptBrackets.ReplaceAllString(place, " "))
	b := tin.NameKey(ptBrackets.ReplaceAllString(town, " "))
	if len(a) < len(b) {
		a, b = b, a
	}
	return a == b || (strings.HasPrefix(a, b+" ") && ptMunicipality.MatchString(a[len(b)+1:]))
}
