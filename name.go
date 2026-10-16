package tin

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// nameElide is punctuation that joins rather than separates, so that
// "A.C.M.E." and "O'Brien" reduce to "ACME" and "OBRIEN" rather than
// splitting into pieces.
var nameElide = regexp.MustCompile(`[.'’]+`)

// NameMatches reports whether two names agree once case, accents,
// punctuation and surrounding whitespace are folded. Case is compared with Unicode case
// folding, so "STRAßE" and "STRAẞE" agree. It compares; it never rewrites.
func NameMatches(a, b string) bool {
	na := NameKey(a)
	return na != "" && strings.EqualFold(na, NameKey(b))
}

// NameKey reduces a name to the key that NameMatches compares. It keeps
// letters and digits in any script, because register names across the EU are
// not ASCII.
func NameKey(s string) string {
	s = norm.NFD.String(strings.ToValidUTF8(strings.TrimSpace(s), ""))
	s = strings.ReplaceAll(s, "&", " AND ")
	s = nameElide.ReplaceAllString(s, "")
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, s)
	return strings.ToUpper(strings.Join(strings.Fields(s), " "))
}
