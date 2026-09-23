package api

import (
	"testing"

	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
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
		return &Result{
			Valid:  true,
			Source: "vies",
			Name:   "ACME GMBH",
			TaxID:  &tax.Identity{Country: "DE", Code: "282741168"},
		}
	}

	t.Run("fills an empty name", func(t *testing.T) {
		party := &org.Party{TaxID: &tax.Identity{Country: "DE", Code: "282741168"}}
		changes, err := valid().ApplyTo(party, ApplyOptions{})
		require.NoError(t, err)
		assert.Equal(t, "ACME GMBH", party.Name)
		assert.True(t, changes.Name)
		assert.False(t, changes.NameMismatch)
		assert.True(t, changes.Any())
	})

	t.Run("keeps a matching name with the user's casing", func(t *testing.T) {
		party := &org.Party{Name: "Acme GmbH", TaxID: &tax.Identity{Country: "DE", Code: "282741168"}}
		changes, err := valid().ApplyTo(party, ApplyOptions{})
		require.NoError(t, err)
		assert.Equal(t, "Acme GmbH", party.Name)
		assert.False(t, changes.Name)
		assert.False(t, changes.NameMismatch)
		assert.False(t, changes.Any())
	})

	t.Run("reports a mismatch without correcting", func(t *testing.T) {
		party := &org.Party{Name: "Other GmbH", TaxID: &tax.Identity{Country: "DE", Code: "282741168"}}
		changes, err := valid().ApplyTo(party, ApplyOptions{})
		require.NoError(t, err)
		assert.Equal(t, "Other GmbH", party.Name)
		assert.False(t, changes.Name)
		assert.True(t, changes.NameMismatch)
		assert.False(t, changes.Any())
	})

	t.Run("corrects a mismatch when asked", func(t *testing.T) {
		party := &org.Party{Name: "Other GmbH", TaxID: &tax.Identity{Country: "DE", Code: "282741168"}}
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

	t.Run("fills an absent tax identity", func(t *testing.T) {
		party := &org.Party{Name: "Acme GmbH"}
		changes, err := valid().ApplyTo(party, ApplyOptions{})
		require.NoError(t, err)
		require.NotNil(t, party.TaxID)
		assert.Equal(t, "282741168", party.TaxID.Code.String())
		assert.True(t, changes.TaxID)
		assert.True(t, changes.Any())
	})

	t.Run("reports a conflicting tax identity without touching it", func(t *testing.T) {
		party := &org.Party{Name: "Acme GmbH", TaxID: &tax.Identity{Country: "DE", Code: "999999999"}}
		changes, err := valid().ApplyTo(party, ApplyOptions{})
		require.NoError(t, err)
		assert.Equal(t, "999999999", party.TaxID.Code.String())
		assert.False(t, changes.TaxID)
		assert.True(t, changes.TaxIDConflict)
	})

	t.Run("appends a missing identity once", func(t *testing.T) {
		res := valid()
		res.Identities = []*org.Identity{{Type: "CRN", Code: "00445790"}}
		party := &org.Party{Name: "Acme GmbH", TaxID: &tax.Identity{Country: "DE", Code: "282741168"}}

		changes, err := res.ApplyTo(party, ApplyOptions{})
		require.NoError(t, err)
		require.Len(t, party.Identities, 1)
		assert.True(t, changes.Identities)

		// A re-run is idempotent.
		changes, err = res.ApplyTo(party, ApplyOptions{})
		require.NoError(t, err)
		require.Len(t, party.Identities, 1)
		assert.False(t, changes.Identities)
		assert.False(t, changes.Any())
	})

	t.Run("reports a conflicting identity without touching it", func(t *testing.T) {
		res := valid()
		res.Identities = []*org.Identity{{Type: "CRN", Code: "00445790"}}
		party := &org.Party{
			Name:       "Acme GmbH",
			TaxID:      &tax.Identity{Country: "DE", Code: "282741168"},
			Identities: []*org.Identity{{Type: "CRN", Code: "99999999"}},
		}
		changes, err := res.ApplyTo(party, ApplyOptions{})
		require.NoError(t, err)
		require.Len(t, party.Identities, 1)
		assert.Equal(t, "99999999", party.Identities[0].Code.String())
		assert.False(t, changes.Identities)
		assert.True(t, changes.IdentityConflict)
	})

	t.Run("address policies", func(t *testing.T) {
		office := &org.Address{Label: "Registered Office", Street: "Musterstr.", Locality: "Berlin"}
		withAddr := func() *Result {
			res := valid()
			res.Address = office
			return res
		}
		existing := func() *org.Party {
			return &org.Party{
				Name:      "Acme GmbH",
				TaxID:     &tax.Identity{Country: "DE", Code: "282741168"},
				Addresses: []*org.Address{{Label: "Billing", Street: "Other St."}},
			}
		}

		t.Run("none leaves addresses untouched", func(t *testing.T) {
			party := existing()
			changes, err := withAddr().ApplyTo(party, ApplyOptions{})
			require.NoError(t, err)
			require.Len(t, party.Addresses, 1)
			assert.False(t, changes.Addresses)
		})

		t.Run("fills empty addresses", func(t *testing.T) {
			party := &org.Party{Name: "Acme GmbH", TaxID: &tax.Identity{Country: "DE", Code: "282741168"}}
			changes, err := withAddr().ApplyTo(party, ApplyOptions{Address: AddressPolicyAppend})
			require.NoError(t, err)
			require.Len(t, party.Addresses, 1)
			assert.True(t, changes.Addresses)
		})

		t.Run("append adds alongside, once", func(t *testing.T) {
			party := existing()
			changes, err := withAddr().ApplyTo(party, ApplyOptions{Address: AddressPolicyAppend})
			require.NoError(t, err)
			require.Len(t, party.Addresses, 2)
			assert.True(t, changes.Addresses)

			changes, err = withAddr().ApplyTo(party, ApplyOptions{Address: AddressPolicyAppend})
			require.NoError(t, err)
			require.Len(t, party.Addresses, 2)
			assert.False(t, changes.Addresses)
		})

		t.Run("replace overwrites the first address", func(t *testing.T) {
			party := existing()
			changes, err := withAddr().ApplyTo(party, ApplyOptions{Address: AddressPolicyReplace})
			require.NoError(t, err)
			require.Len(t, party.Addresses, 1)
			assert.Equal(t, "Musterstr.", party.Addresses[0].Street)
			assert.True(t, changes.Addresses)
		})

		t.Run("no structured address applies nothing", func(t *testing.T) {
			party := existing()
			changes, err := valid().ApplyTo(party, ApplyOptions{Address: AddressPolicyReplace})
			require.NoError(t, err)
			assert.Equal(t, "Other St.", party.Addresses[0].Street)
			assert.False(t, changes.Addresses)
		})
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
