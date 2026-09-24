package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNameMatches(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{name: "identical", a: "ACME GMBH", b: "ACME GMBH", want: true},
		{name: "case folded", a: "ACME GMBH", b: "Acme GmbH", want: true},
		{name: "punctuation elided", a: "A.C.M.E. GMBH", b: "ACME GmbH", want: true},
		{name: "apostrophes elided", a: "O'BRIEN LTD", b: "O’Brien Ltd", want: true},
		{name: "ampersand folded", a: "SMITH & SONS", b: "Smith and Sons", want: true},
		{name: "whitespace folded", a: "  ACME   GMBH ", b: "Acme GmbH", want: true},
		{name: "separating punctuation", a: "ACME-GMBH", b: "Acme GmbH", want: true},
		{name: "accented letters kept", a: "MUÑOZ SL", b: "Muñoz SL", want: true},
		{name: "different names", a: "ACME GMBH", b: "Other GmbH", want: false},
		{name: "different accents differ", a: "MUÑOZ SL", b: "Munoz SL", want: false},
		{name: "empty never matches", a: "", b: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NameMatches(tt.a, tt.b))
		})
	}
}
