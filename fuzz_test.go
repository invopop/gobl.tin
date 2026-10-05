package tin

import (
	"strings"
	"testing"
	"unicode"
)

func FuzzNormalizeName(f *testing.F) {
	for _, seed := range []string{"", "ACME GMBH", "A.C.M.E. GmbH", "Smith & Sons", "  Muñoz   SL ", "O’Brien-Ltd", "---"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		n := normalizeName(s)
		if n != normalizeName(n) {
			t.Fatalf("not idempotent: %q -> %q -> %q", s, n, normalizeName(n))
		}
		if n != strings.TrimSpace(n) || strings.Contains(n, "  ") {
			t.Fatalf("whitespace not folded: %q", n)
		}
		for _, r := range n {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != ' ' {
				t.Fatalf("unexpected rune %q in %q", r, n)
			}
		}
		if NameMatches(s, s) != (n != "") {
			t.Fatalf("NameMatches(s, s) disagrees with emptiness for %q", s)
		}
	})
}
