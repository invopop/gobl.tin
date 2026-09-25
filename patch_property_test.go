package tin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPatchInvariants applies every policy combination to a set of parties
// and records and checks what a patch may never do.
func TestPatchInvariants(t *testing.T) {
	tid := &tax.Identity{Country: "DE", Code: "282741168"}
	parties := []*org.Party{
		{TaxID: tid},
		{Name: "Acme Trading", TaxID: tid},
		{Name: "Acme Trading", TaxID: tid, Identities: []*org.Identity{{Country: "DE", Type: "HRB", Code: "1"}}},
		{Name: "Acme Trading", TaxID: tid, Addresses: []*org.Address{{Label: "Billing", Locality: "Berlin"}}},
		{Name: "Acme Trading", TaxID: tid,
			Identities: []*org.Identity{{Country: "DE", Type: "HRB", Code: "1"}, {Country: "DE", Type: "EORI", Code: "old"}},
			Addresses:  []*org.Address{{Locality: "Berlin"}, {Label: "Registered Office", Locality: "Potsdam"}}},
	}
	records := []*Record{
		nil,
		{Name: "ACME TRADING GMBH"},
		{Name: "ACME TRADING GMBH", Identities: []*org.Identity{{Country: "DE", Type: "EORI", Code: "new"}, {Country: "DE", Type: "HRB", Code: "1"}}},
		{Addresses: []*org.Address{{Label: "Registered Office", Locality: "Munich"}, {Locality: "Berlin"}}},
	}
	names := []NamePolicy{"", NamePolicyFill, NamePolicyPreferRegister, NamePolicyKeep}
	identities := []IdentitiesPolicy{"", IdentitiesPolicyAddMissing, IdentitiesPolicyKeep}
	addresses := []AddressPolicy{"", AddressPolicyNone, AddressPolicyAppend, AddressPolicyReplace}

	for _, party := range parties {
		for _, rec := range records {
			report := &Report{Checks: []*Check{{Path: PathTaxID, Status: StatusValid, Source: "vies", Record: rec}}}
			for _, n := range names {
				for _, i := range identities {
					for _, a := range addresses {
						policy := Policy{Name: n, Identities: i, Addresses: a}
						patch, err := Patch(party, report, policy)
						require.NoError(t, err)
						got := applyPatch(t, party, patch)

						assert.Equal(t, party.TaxID, got.TaxID, "tax_id never changes")
						for _, id := range party.Identities {
							assert.Contains(t, got.Identities, id, "no identity is removed")
						}
						if a != AddressPolicyReplace {
							for _, addr := range party.Addresses {
								assert.Contains(t, got.Addresses, addr, "no address is removed except under replace")
							}
						}
						if n == NamePolicyKeep || party.Name != "" && (n == "" || n == NamePolicyFill) {
							assert.Equal(t, party.Name, got.Name, "name kept under %q", n)
						}
					}
				}
			}
		}
	}
}

// TestPatchIdempotent applies a patch, verifies the patched party against the
// same record, and expects the second patch to be empty.
func TestPatchIdempotent(t *testing.T) {
	rec := &Record{
		Name:       "ACME TRADING GMBH",
		Identities: []*org.Identity{{Country: "DE", Type: "EORI", Code: "DE123"}},
		Addresses:  []*org.Address{{Label: "Registered Office", Locality: "Berlin", Country: "DE"}},
	}
	fake := &fakeVerifier{source: "vies", checks: map[cbc.Code]*Answer{"282741168": valid("vies", rec)}}
	c := New(fake)
	party := &org.Party{
		Name:       "Acme Trading",
		TaxID:      &tax.Identity{Country: "DE", Code: "282741168"},
		Identities: []*org.Identity{{Country: "DE", Type: "HRB", Code: "12345"}},
		Addresses:  []*org.Address{{Label: "Billing", Locality: "Munich"}},
	}
	policy := Policy{Name: NamePolicyPreferRegister, Identities: IdentitiesPolicyAddMissing, Addresses: AddressPolicyAppend}

	report, err := c.Verify(context.Background(), party)
	require.NoError(t, err)
	first, err := Patch(party, report, policy)
	require.NoError(t, err)
	assert.NotEqual(t, `{}`, string(first))

	patched := applyPatch(t, party, first)
	report, err = c.Verify(context.Background(), patched)
	require.NoError(t, err)
	assert.Empty(t, report.Checks[0].Mismatches)
	second, err := Patch(patched, report, policy)
	require.NoError(t, err)
	assert.Equal(t, json.RawMessage(`{}`), second)
}
