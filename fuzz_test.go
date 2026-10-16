package tin

import (
	"strings"
	"testing"
	"unicode"
)

func FuzzNameKey(f *testing.F) {
	for _, seed := range []string{"", "ACME GMBH", "A.C.M.E. GmbH", "Smith & Sons", "  Muñoz   SL ", "Telefónica, S.A.", "ǰ", "O’Brien-Ltd", "---"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		n := NameKey(s)
		if n != NameKey(n) {
			t.Fatalf("not idempotent: %q -> %q -> %q", s, n, NameKey(n))
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
