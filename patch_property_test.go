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

// TestPatchInvariants patches a set of parties with a set of records and
// checks what a patch may never do.
func TestPatchInvariants(t *testing.T) {
	tid := &tax.Identity{Country: "DE", Code: "282741168"}
	parties := []*org.Party{
		{TaxID: tid},
		{Name: "Acme Trading", TaxID: tid},
		{Name: "Acme Trading", TaxID: tid, Addresses: []*org.Address{{Label: "Billing", Locality: "Berlin"}}},
		{Name: "Acme Trading", TaxID: tid,
			Addresses: []*org.Address{{Locality: "Berlin"}, {Label: "Registered Office", Locality: "Potsdam"}}},
	}
	records := []*Record{
		nil,
		{Name: "ACME TRADING GMBH"},
		{Name: "ACME TRADING"},
		{Address: &org.Address{Label: "Registered Office", Locality: "Munich"}},
	}

	for _, party := range parties {
		for _, rec := range records {
			report := &Report{Checks: []*Check{{Path: PathTaxID, TaxID: party.TaxID, Status: StatusValid, Source: "vies", Record: rec}}}
			patch, err := Patch(party, report)
			require.NoError(t, err)
			got := applyPatch(t, party, patch)

			assert.Equal(t, party.TaxID, got.TaxID, "tax_id never changes")
			if len(party.Addresses) > 1 {
				assert.Equal(t, party.Addresses[1:], got.Addresses[1:], "only the first address may change")
			}
			if rec == nil || rec.Address == nil {
				assert.Equal(t, party.Addresses, got.Addresses, "no structured address, no address change")
			}
			if rec != nil && rec.Name != "" {
				assert.Equal(t, rec.Name, got.Name, "a registered name is always the party's after the patch")
			} else {
				assert.Equal(t, party.Name, got.Name, "no registered name keeps the party's")
			}
		}
	}
}

// TestPatchIdempotent applies a patch, verifies the patched party against the
// same record, and expects the second patch to be empty.
func TestPatchIdempotent(t *testing.T) {
	rec := &Record{
		Name:    "ACME TRADING GMBH",
		Address: &org.Address{Street: "Hauptstrasse", Number: "1", Locality: "Berlin", Country: "DE"},
	}
	fake := &fakeVerifier{source: "vies", checks: map[cbc.Code]*Answer{"282741168": valid("vies", rec)}}
	c := New(fake)
	party := &org.Party{
		Name:      "Acme",
		TaxID:     &tax.Identity{Country: "DE", Code: "282741168"},
		Addresses: []*org.Address{{Label: "Billing", Locality: "Munich"}},
	}

	report, err := c.Verify(context.Background(), party)
	require.NoError(t, err)
	first, err := Patch(party, report)
	require.NoError(t, err)
	assert.NotEqual(t, `{}`, string(first))

	patched := applyPatch(t, party, first)
	report, err = c.Verify(context.Background(), patched)
	require.NoError(t, err)
	assert.Empty(t, report.Checks[0].Mismatches)
	second, err := Patch(patched, report)
	require.NoError(t, err)
	assert.Equal(t, json.RawMessage(`{}`), second)
}
