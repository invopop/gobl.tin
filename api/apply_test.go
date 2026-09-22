package api

import (
	"testing"

	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestResultApplyTo(t *testing.T) {
	valid := func() *Result {
		return &Result{Valid: true, Name: "ACME GMBH", Address: "MUSTERSTR. 1, 10115 BERLIN", Source: "vies"}
	}

	t.Run("fills an empty name", func(t *testing.T) {
		party := &org.Party{}
		changes, err := valid().ApplyTo(party, ApplyOptions{})
		require.NoError(t, err)
		assert.Equal(t, "ACME GMBH", party.Name)
		assert.True(t, changes.Name)
		assert.False(t, changes.NameMismatch)
		assert.True(t, changes.Any())
	})

	t.Run("keeps a matching name with the user's casing", func(t *testing.T) {
		party := &org.Party{Name: "Acme GmbH"}
		changes, err := valid().ApplyTo(party, ApplyOptions{})
		require.NoError(t, err)
		assert.Equal(t, "Acme GmbH", party.Name)
		assert.False(t, changes.Name)
		assert.False(t, changes.NameMismatch)
		assert.False(t, changes.Any())
	})

	t.Run("reports a mismatch without correcting", func(t *testing.T) {
		party := &org.Party{Name: "Other GmbH"}
		changes, err := valid().ApplyTo(party, ApplyOptions{})
		require.NoError(t, err)
		assert.Equal(t, "Other GmbH", party.Name)
		assert.False(t, changes.Name)
		assert.True(t, changes.NameMismatch)
		assert.False(t, changes.Any())
	})

	t.Run("corrects a mismatch when asked", func(t *testing.T) {
		party := &org.Party{Name: "Other GmbH"}
		changes, err := valid().ApplyTo(party, ApplyOptions{CorrectName: true})
		require.NoError(t, err)
		assert.Equal(t, "ACME GMBH", party.Name)
		assert.True(t, changes.Name)
		assert.True(t, changes.NameMismatch)
		assert.True(t, changes.Any())
	})

	t.Run("masked name applies nothing", func(t *testing.T) {
		res := &Result{Valid: true, Source: "vies"}
		party := &org.Party{Name: "Acme GmbH"}
		changes, err := res.ApplyTo(party, ApplyOptions{CorrectName: true})
		require.NoError(t, err)
		assert.Equal(t, "Acme GmbH", party.Name)
		assert.False(t, changes.Any())
		assert.False(t, changes.NameMismatch)
	})

	t.Run("never writes the address", func(t *testing.T) {
		party := &org.Party{}
		_, err := valid().ApplyTo(party, ApplyOptions{})
		require.NoError(t, err)
		assert.Empty(t, party.Addresses)
	})

	t.Run("invalid result is an input error", func(t *testing.T) {
		res := &Result{Valid: false, Name: "ACME GMBH", Source: "vies"}
		_, err := res.ApplyTo(&org.Party{}, ApplyOptions{})
		assert.ErrorIs(t, err, ErrInput)
	})

	t.Run("nil result is an input error", func(t *testing.T) {
		var res *Result
		_, err := res.ApplyTo(&org.Party{}, ApplyOptions{})
		assert.ErrorIs(t, err, ErrInput)
	})

	t.Run("nil party is an input error", func(t *testing.T) {
		_, err := valid().ApplyTo(nil, ApplyOptions{})
		assert.ErrorIs(t, err, ErrInput)
	})
}
