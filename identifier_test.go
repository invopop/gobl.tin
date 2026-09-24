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

	t.Run("party without identifiers", func(t *testing.T) {
		assert.Empty(t, walk(&org.Party{Name: "Acme"}))
	})

	t.Run("tax id without code is skipped", func(t *testing.T) {
		assert.Empty(t, walk(&org.Party{TaxID: &tax.Identity{Country: "US"}}))
	})

	t.Run("tax id first, then identities in order", func(t *testing.T) {
		party := &org.Party{
			TaxID: &tax.Identity{Country: "DE", Code: "282741168"},
			Identities: []*org.Identity{
				{Country: "DE", Type: "HRB", Code: "12345"},
				nil,
				{Country: "GB", Type: "CRN", Code: "00445790"},
			},
		}
		ids := walk(party)
		require.Len(t, ids, 3)

		assert.Equal(t, "tax_id", ids[0].Path)
		assert.True(t, ids[0].IsTaxID())
		assert.Equal(t, "DE", ids[0].Country.String())
		assert.Empty(t, ids[0].Type)
		assert.Equal(t, "282741168", ids[0].Code.String())
		require.NotNil(t, ids[0].taxID)
		assert.Nil(t, ids[0].identity)

		assert.Equal(t, "identities[0]", ids[1].Path)
		assert.False(t, ids[1].IsTaxID())
		assert.Equal(t, "DE", ids[1].Country.String())
		assert.Equal(t, "HRB", ids[1].Type.String())
		assert.Equal(t, "12345", ids[1].Code.String())
		assert.Nil(t, ids[1].taxID)
		require.NotNil(t, ids[1].identity)

		assert.Equal(t, "identities[2]", ids[2].Path, "the nil entry keeps its index")
		assert.Equal(t, "GB", ids[2].Country.String())
		assert.Equal(t, "CRN", ids[2].Type.String())
	})

	t.Run("normalizes copies and leaves the party untouched", func(t *testing.T) {
		party := &org.Party{
			TaxID: &tax.Identity{Country: "DE", Code: " de 282-741.168 "},
			Identities: []*org.Identity{
				{Country: "GB", Type: "CRN", Code: " sc123456 "},
			},
		}
		ids := walk(party)
		require.Len(t, ids, 2)

		assert.Equal(t, "282741168", ids[0].Code.String())
		assert.Equal(t, "282741168", ids[0].taxID.Code.String())
		assert.Equal(t, " de 282-741.168 ", party.TaxID.Code.String())
		assert.NotSame(t, party.TaxID, ids[0].taxID)

		assert.Equal(t, "SC123456", ids[1].Code.String())
		assert.Equal(t, "SC123456", ids[1].identity.Code.String())
		assert.Equal(t, " sc123456 ", party.Identities[0].Code.String())
		assert.NotSame(t, party.Identities[0], ids[1].identity)
	})

	t.Run("identity with key and no type", func(t *testing.T) {
		party := &org.Party{
			Identities: []*org.Identity{{Key: "other", Code: "abc"}},
		}
		ids := walk(party)
		require.Len(t, ids, 1)
		assert.Equal(t, "other", ids[0].Key.String())
		assert.Empty(t, ids[0].Type)
		assert.Empty(t, ids[0].Country)
		assert.Equal(t, "ABC", ids[0].Code.String())
	})
}
