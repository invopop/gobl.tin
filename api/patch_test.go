package api

import (
	"encoding/json"
	"testing"

	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mergePatch applies an RFC 7396 merge patch to a decoded JSON value.
func mergePatch(target, patch any) any {
	pm, ok := patch.(map[string]any)
	if !ok {
		return patch
	}
	tm, ok := target.(map[string]any)
	if !ok {
		tm = map[string]any{}
	}
	for k, v := range pm {
		if v == nil {
			delete(tm, k)
			continue
		}
		tm[k] = mergePatch(tm[k], v)
	}
	return tm
}

// applyPatch merges the patch into the party's JSON and returns the result
// as a party.
func applyPatch(t *testing.T, party *org.Party, patch json.RawMessage) *org.Party {
	t.Helper()
	var doc, p any
	data, err := json.Marshal(party)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &doc))
	require.NoError(t, json.Unmarshal(patch, &p))
	merged, err := json.Marshal(mergePatch(doc, p))
	require.NoError(t, err)
	out := new(org.Party)
	require.NoError(t, json.Unmarshal(merged, out))
	return out
}

func report(party *org.Party, checks ...*Check) *Verification {
	v := NewVerification(party)
	v.Checks = checks
	return v
}

func validCheck(rec *Record) *Check {
	return &Check{Path: PathTaxID, Status: StatusValid, Source: "vies", Record: rec}
}

func TestPatch(t *testing.T) {
	tid := &tax.Identity{Country: "DE", Code: "282741168"}

	t.Run("report without party is an input error", func(t *testing.T) {
		v := &Verification{Checks: []*Check{validCheck(&Record{Name: "ACME GMBH"})}}
		_, err := v.Patch(Policy{})
		assert.ErrorIs(t, err, ErrInput)

		var nilReport *Verification
		_, err = nilReport.Patch(Policy{})
		assert.ErrorIs(t, err, ErrInput)
	})

	t.Run("unknown policy values are input errors", func(t *testing.T) {
		v := report(&org.Party{TaxID: tid}, validCheck(&Record{Name: "ACME GMBH"}))
		_, err := v.Patch(Policy{Name: "shout"})
		assert.ErrorIs(t, err, ErrInput)
		_, err = v.Patch(Policy{Identities: "merge"})
		assert.ErrorIs(t, err, ErrInput)
		_, err = v.Patch(Policy{Addresses: "first"})
		assert.ErrorIs(t, err, ErrInput)
	})

	t.Run("nothing to write is an empty object", func(t *testing.T) {
		party := &org.Party{Name: "Acme GmbH", TaxID: tid}
		v := report(party, validCheck(&Record{Name: "ACME GMBH"}))
		patch, err := v.Patch(Policy{})
		require.NoError(t, err)
		assert.Equal(t, `{}`, string(patch))
		assert.Equal(t, party, applyPatch(t, party, patch))
	})

	t.Run("invalid and unverified records are ignored", func(t *testing.T) {
		party := &org.Party{TaxID: tid}
		v := report(party,
			&Check{Path: PathTaxID, Status: StatusInvalid, Record: &Record{Name: "WRONG"}},
			&Check{Path: "identities[0]", Status: StatusUnverified, Record: &Record{Name: "WRONG"}},
		)
		patch, err := v.Patch(Policy{})
		require.NoError(t, err)
		assert.Equal(t, `{}`, string(patch))
	})

	t.Run("name", func(t *testing.T) {
		rec := &Record{Name: "ACME TRADING GMBH"}

		t.Run("fill writes an empty name", func(t *testing.T) {
			party := &org.Party{TaxID: tid}
			patch, err := report(party, validCheck(rec)).Patch(Policy{})
			require.NoError(t, err)
			assert.Equal(t, `{"name":"ACME TRADING GMBH"}`, string(patch))
			got := applyPatch(t, party, patch)
			assert.Equal(t, "ACME TRADING GMBH", got.Name)
			assert.Equal(t, tid, got.TaxID)
		})

		t.Run("fill keeps a present name", func(t *testing.T) {
			party := &org.Party{Name: "Acme Trading", TaxID: tid}
			patch, err := report(party, validCheck(rec)).Patch(Policy{Name: NamePolicyFill})
			require.NoError(t, err)
			assert.Equal(t, `{}`, string(patch))
		})

		t.Run("prefer-register rewrites a different name", func(t *testing.T) {
			party := &org.Party{Name: "Acme Trading", TaxID: tid}
			patch, err := report(party, validCheck(rec)).Patch(Policy{Name: NamePolicyPreferRegister})
			require.NoError(t, err)
			assert.Equal(t, `{"name":"ACME TRADING GMBH"}`, string(patch))
			assert.Equal(t, "ACME TRADING GMBH", applyPatch(t, party, patch).Name)
		})

		t.Run("prefer-register rewrites a case difference", func(t *testing.T) {
			party := &org.Party{Name: "Acme Trading GmbH", TaxID: tid}
			patch, err := report(party, validCheck(rec)).Patch(Policy{Name: NamePolicyPreferRegister})
			require.NoError(t, err)
			assert.Equal(t, `{"name":"ACME TRADING GMBH"}`, string(patch))
		})

		t.Run("prefer-register with an equal name writes nothing", func(t *testing.T) {
			party := &org.Party{Name: "ACME TRADING GMBH", TaxID: tid}
			patch, err := report(party, validCheck(rec)).Patch(Policy{Name: NamePolicyPreferRegister})
			require.NoError(t, err)
			assert.Equal(t, `{}`, string(patch))
		})

		t.Run("prefer-register with a masked name writes nothing", func(t *testing.T) {
			party := &org.Party{Name: "Acme Trading", TaxID: tid}
			patch, err := report(party, validCheck(nil)).Patch(Policy{Name: NamePolicyPreferRegister})
			require.NoError(t, err)
			assert.Equal(t, `{}`, string(patch))
		})

		t.Run("keep never writes", func(t *testing.T) {
			party := &org.Party{TaxID: tid}
			patch, err := report(party, validCheck(rec)).Patch(Policy{Name: NamePolicyKeep})
			require.NoError(t, err)
			assert.Equal(t, `{}`, string(patch))
		})

		t.Run("first valid record with a name wins", func(t *testing.T) {
			party := &org.Party{TaxID: tid}
			v := report(party,
				validCheck(nil),
				&Check{Path: "identities[0]", Status: StatusValid, Record: &Record{Name: "FIRST"}},
				&Check{Path: "identities[1]", Status: StatusValid, Record: &Record{Name: "SECOND"}},
			)
			patch, err := v.Patch(Policy{})
			require.NoError(t, err)
			assert.Equal(t, `{"name":"FIRST"}`, string(patch))
		})
	})

	t.Run("identities", func(t *testing.T) {
		rec := &Record{Identities: []*org.Identity{
			{Country: "GB", Type: "CRN", Code: "00445790"},
			{Country: "GB", Type: "UTR", Code: "1234567890"},
		}}

		t.Run("add-missing appends what the party lacks as a full array", func(t *testing.T) {
			party := &org.Party{
				Name:       "Acme Ltd",
				Identities: []*org.Identity{{Country: "GB", Type: "CRN", Code: "00445790"}},
			}
			patch, err := report(party, validCheck(rec)).Patch(Policy{})
			require.NoError(t, err)
			assert.Equal(t, `{"identities":[{"country":"GB","type":"CRN","code":"00445790"},{"country":"GB","type":"UTR","code":"1234567890"}]}`, string(patch))
			got := applyPatch(t, party, patch)
			require.Len(t, got.Identities, 2)
			assert.Equal(t, "UTR", got.Identities[1].Type.String())
			assert.Equal(t, "Acme Ltd", got.Name)
		})

		t.Run("add-missing fills an empty list", func(t *testing.T) {
			party := &org.Party{Name: "Acme Ltd"}
			patch, err := report(party, validCheck(rec)).Patch(Policy{Identities: IdentitiesPolicyAddMissing})
			require.NoError(t, err)
			assert.Equal(t, `{"identities":[{"country":"GB","type":"CRN","code":"00445790"},{"country":"GB","type":"UTR","code":"1234567890"}]}`, string(patch))
			assert.Len(t, applyPatch(t, party, patch).Identities, 2)
		})

		t.Run("conflicts are never written", func(t *testing.T) {
			party := &org.Party{Identities: []*org.Identity{
				{Country: "GB", Type: "CRN", Code: "00445790"},
				{Country: "GB", Type: "UTR", Code: "0000000000"},
			}}
			patch, err := report(party, validCheck(rec)).Patch(Policy{})
			require.NoError(t, err)
			assert.Equal(t, `{}`, string(patch))
		})

		t.Run("a different country is not the same kind", func(t *testing.T) {
			party := &org.Party{Identities: []*org.Identity{{Country: "IE", Type: "CRN", Code: "123"}}}
			single := &Record{Identities: rec.Identities[:1]}
			patch, err := report(party, validCheck(single)).Patch(Policy{})
			require.NoError(t, err)
			assert.Equal(t, `{"identities":[{"country":"IE","type":"CRN","code":"123"},{"country":"GB","type":"CRN","code":"00445790"}]}`, string(patch))
		})

		t.Run("keep never writes", func(t *testing.T) {
			party := &org.Party{}
			patch, err := report(party, validCheck(rec)).Patch(Policy{Identities: IdentitiesPolicyKeep})
			require.NoError(t, err)
			assert.Equal(t, `{}`, string(patch))
		})
	})

	t.Run("addresses", func(t *testing.T) {
		office := &org.Address{Label: "Registered Office", Street: "Musterstr.", Locality: "Berlin", Country: "DE"}
		rec := &Record{Addresses: []*org.Address{office}}
		billing := &org.Address{Label: "Billing", Street: "Other St.", Country: "DE"}

		t.Run("none writes nothing", func(t *testing.T) {
			party := &org.Party{Addresses: []*org.Address{billing}}
			patch, err := report(party, validCheck(rec)).Patch(Policy{})
			require.NoError(t, err)
			assert.Equal(t, `{}`, string(patch))
		})

		t.Run("append adds alongside as a full array", func(t *testing.T) {
			party := &org.Party{Addresses: []*org.Address{billing}}
			patch, err := report(party, validCheck(rec)).Patch(Policy{Addresses: AddressPolicyAppend})
			require.NoError(t, err)
			assert.Equal(t, `{"addresses":[{"label":"Billing","street":"Other St.","country":"DE"},{"label":"Registered Office","street":"Musterstr.","locality":"Berlin","country":"DE"}]}`, string(patch))
			got := applyPatch(t, party, patch)
			require.Len(t, got.Addresses, 2)
			assert.Equal(t, "Berlin", got.Addresses[1].Locality)
		})

		t.Run("append skips a present label", func(t *testing.T) {
			party := &org.Party{Addresses: []*org.Address{{Label: "Registered Office", Street: "Old St."}}}
			patch, err := report(party, validCheck(rec)).Patch(Policy{Addresses: AddressPolicyAppend})
			require.NoError(t, err)
			assert.Equal(t, `{}`, string(patch))
		})

		t.Run("append compares unlabelled addresses field by field", func(t *testing.T) {
			plain := &org.Address{Street: "Musterstr.", Locality: "Berlin", Country: "DE"}
			party := &org.Party{Addresses: []*org.Address{{Street: "Musterstr.", Locality: "Berlin", Country: "DE"}}}
			patch, err := report(party, validCheck(&Record{Addresses: []*org.Address{plain}})).Patch(Policy{Addresses: AddressPolicyAppend})
			require.NoError(t, err)
			assert.Equal(t, `{}`, string(patch))
		})

		t.Run("replace writes the record addresses", func(t *testing.T) {
			party := &org.Party{Addresses: []*org.Address{billing}}
			patch, err := report(party, validCheck(rec)).Patch(Policy{Addresses: AddressPolicyReplace})
			require.NoError(t, err)
			assert.Equal(t, `{"addresses":[{"label":"Registered Office","street":"Musterstr.","locality":"Berlin","country":"DE"}]}`, string(patch))
			got := applyPatch(t, party, patch)
			require.Len(t, got.Addresses, 1)
			assert.Equal(t, "Registered Office", got.Addresses[0].Label)
		})

		t.Run("replace without record addresses writes nothing", func(t *testing.T) {
			party := &org.Party{Addresses: []*org.Address{billing}}
			patch, err := report(party, validCheck(&Record{Name: "ACME"})).Patch(Policy{Name: NamePolicyKeep, Addresses: AddressPolicyReplace})
			require.NoError(t, err)
			assert.Equal(t, `{}`, string(patch))
		})
	})

	t.Run("all fields together round trip", func(t *testing.T) {
		party := &org.Party{
			TaxID:      tid,
			Identities: []*org.Identity{{Country: "DE", Type: "HRB", Code: "12345"}},
		}
		rec := &Record{
			Name:       "ACME GMBH",
			Identities: []*org.Identity{{Country: "DE", Type: "EORI", Code: "DE123"}},
			Addresses:  []*org.Address{{Label: "Registered Office", Locality: "Berlin", Country: "DE"}},
		}
		patch, err := report(party, validCheck(rec)).Patch(Policy{Addresses: AddressPolicyAppend})
		require.NoError(t, err)
		assert.Equal(t, `{"addresses":[{"label":"Registered Office","locality":"Berlin","country":"DE"}],"identities":[{"country":"DE","type":"HRB","code":"12345"},{"country":"DE","type":"EORI","code":"DE123"}],"name":"ACME GMBH"}`, string(patch))
		got := applyPatch(t, party, patch)
		assert.Equal(t, "ACME GMBH", got.Name)
		assert.Equal(t, tid, got.TaxID)
		assert.Len(t, got.Identities, 2)
		assert.Len(t, got.Addresses, 1)
	})
}
