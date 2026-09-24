package api

import (
	"regexp"
	"strings"
	"unicode"
)

// nameElide is punctuation that joins rather than separates, so that
// "A.C.M.E." and "O'Brien" reduce to "ACME" and "OBRIEN" rather than
// splitting into pieces.
var nameElide = regexp.MustCompile(`[.'’]+`)

// NameMatches reports whether two names agree once case, punctuation and
// surrounding whitespace are folded. It compares; it never rewrites.
func NameMatches(a, b string) bool {
	na := normalizeName(a)
	return na != "" && na == normalizeName(b)
}

// normalizeName reduces a name to a comparison key. It keeps letters and
// digits in any script, because register names across the EU are not ASCII.
func normalizeName(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "&", " AND ")
	s = nameElide.ReplaceAllString(s, "")
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, s)
	return strings.Join(strings.Fields(s), " ")
}
