package tin

import (
	"strings"
	"testing"
	"unicode"

	"github.com/invopop/gobl/cbc"
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

func FuzzNormalizeCode(f *testing.F) {
	for _, seed := range []string{"", " sc123456 ", "00445790", "de-282", "ñ"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		c := normalizeCode(cbc.Code(s))
		if c != normalizeCode(c) {
			t.Fatalf("not idempotent: %q -> %q", s, c)
		}
		if c.String() != strings.TrimSpace(c.String()) {
			t.Fatalf("not trimmed: %q", c)
		}
		if c.String() != strings.ToUpper(c.String()) {
			t.Fatalf("not upper cased: %q", c)
		}
	})
}
