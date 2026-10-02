package tin

import (
	"testing"

	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalk(t *testing.T) {
	t.Run("nil party", func(t *testing.T) {
		assert.Nil(t, walk(nil))
	})

	t.Run("party without a tax id", func(t *testing.T) {
		assert.Empty(t, walk(&org.Party{Name: "Acme"}))
	})

	t.Run("tax id without code is skipped", func(t *testing.T) {
		assert.Empty(t, walk(&org.Party{TaxID: &tax.Identity{Country: "US"}}))
	})

	t.Run("a code that normalizes to empty is skipped", func(t *testing.T) {
		assert.Empty(t, walk(&org.Party{TaxID: &tax.Identity{Country: "DE", Code: "DE"}}))
	})

	t.Run("identities are not walked", func(t *testing.T) {
		party := &org.Party{
			TaxID:      &tax.Identity{Country: "DE", Code: "282741168"},
			Identities: []*org.Identity{{Country: "DE", Type: "HRB", Code: "12345"}},
		}
		ids := walk(party)
		require.Len(t, ids, 1)
		assert.Equal(t, "/tax_id", ids[0].Path)
		assert.Equal(t, "DE", ids[0].Country.String())
		assert.Equal(t, "282741168", ids[0].Code.String())
	})

	t.Run("upper-cases a lower-case country", func(t *testing.T) {
		ids := walk(&org.Party{TaxID: &tax.Identity{Country: "de", Code: "DE282741168"}})
		require.Len(t, ids, 1)
		assert.Equal(t, "DE", ids[0].Country.String())
		assert.Equal(t, "282741168", ids[0].Code.String(), "the prefix is stripped once the country matches")
	})

	t.Run("normalizes a copy and leaves the party untouched", func(t *testing.T) {
		party := &org.Party{TaxID: &tax.Identity{Country: "DE", Code: " de 282-741.168 "}}
		ids := walk(party)
		require.Len(t, ids, 1)
		assert.Equal(t, "282741168", ids[0].Code.String())
		assert.Equal(t, "282741168", ids[0].taxID.Code.String())
		assert.Equal(t, " de 282-741.168 ", party.TaxID.Code.String())
		assert.NotSame(t, party.TaxID, ids[0].taxID)
	})
}
