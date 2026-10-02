package tin

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

// patchOf builds a report from the checks and patches the party with it.
func patchOf(party *org.Party, checks ...*Check) (json.RawMessage, error) {
	return Patch(party, &Report{Checks: checks})
}

func validCheck(rec *Record) *Check {
	return &Check{Path: PathTaxID, Status: StatusValid, Source: "vies", Record: rec}
}

func TestPatch(t *testing.T) {
	tid := &tax.Identity{Country: "DE", Code: "282741168"}
	office := &org.Address{Street: "Hauptstrasse", Number: "1", Code: "10115", Locality: "Berlin", Country: "DE"}
	billing := &org.Address{Label: "Billing", Locality: "Munich", Country: "DE"}

	tests := []struct {
		name   string
		party  *org.Party
		checks []*Check
		want   string
	}{
		{
			name:   "fills a missing name",
			party:  &org.Party{TaxID: tid},
			checks: []*Check{validCheck(&Record{Name: "ACME TRADING GMBH"})},
			want:   `{"name":"ACME TRADING GMBH"}`,
		},
		{
			name:   "replaces a different name",
			party:  &org.Party{Name: "Acme", TaxID: tid},
			checks: []*Check{validCheck(&Record{Name: "ACME TRADING GMBH"})},
			want:   `{"name":"ACME TRADING GMBH"}`,
		},
		{
			name:   "replaces a name that differs only by case or punctuation",
			party:  &org.Party{Name: "Acme Trading GmbH.", TaxID: tid},
			checks: []*Check{validCheck(&Record{Name: "ACME TRADING GMBH"})},
			want:   `{"name":"ACME TRADING GMBH"}`,
		},
		{
			name:   "writes nothing when the name is already the registered one",
			party:  &org.Party{Name: "ACME TRADING GMBH", TaxID: tid},
			checks: []*Check{validCheck(&Record{Name: "ACME TRADING GMBH"})},
			want:   `{}`,
		},
		{
			name:   "adds the registered address",
			party:  &org.Party{Name: "ACME TRADING GMBH", TaxID: tid},
			checks: []*Check{validCheck(&Record{Address: office})},
			want:   `{"addresses":[{"num":"1","street":"Hauptstrasse","locality":"Berlin","code":"10115","country":"DE"}]}`,
		},
		{
			name:   "puts the registered address first and keeps the others",
			party:  &org.Party{TaxID: tid, Addresses: []*org.Address{billing, {Label: "Warehouse", Locality: "Hamburg"}}},
			checks: []*Check{validCheck(&Record{Address: office})},
			want: `{"addresses":[{"num":"1","street":"Hauptstrasse","locality":"Berlin","code":"10115","country":"DE"},` +
				`{"label":"Warehouse","locality":"Hamburg"}]}`,
		},
		{
			name:   "writes nothing when the registered address is already first",
			party:  &org.Party{TaxID: tid, Addresses: []*org.Address{office, billing}},
			checks: []*Check{validCheck(&Record{Address: office})},
			want:   `{}`,
		},
		{
			name:   "reads only valid records",
			party:  &org.Party{TaxID: tid},
			checks: []*Check{{Path: PathTaxID, Status: StatusInvalid, Record: &Record{Name: "X", Address: office}}},
			want:   `{}`,
		},
		{
			name:   "nothing to write",
			party:  &org.Party{TaxID: tid},
			checks: []*Check{validCheck(nil)},
			want:   `{}`,
		},
	}
	edge := []struct {
		name   string
		party  *org.Party
		checks []*Check
		want   string
	}{
		{"empty registered name writes nothing", &org.Party{Name: "Acme", TaxID: tid},
			[]*Check{validCheck(&Record{Name: ""})}, `{}`},
		{"blank registered name writes nothing", &org.Party{Name: "Acme", TaxID: tid},
			[]*Check{validCheck(&Record{Name: "   "})}, `{}`},
		{"placeholder name writes nothing", &org.Party{Name: "Acme", TaxID: tid},
			[]*Check{validCheck(&Record{Name: "---"})}, `{}`},
		{"a placeholder lets a later record's name through", &org.Party{TaxID: tid},
			[]*Check{validCheck(&Record{Name: "- - -"}), validCheck(&Record{Name: "ACME"})}, `{"name":"ACME"}`},
		{"registered name is trimmed", &org.Party{Name: "Acme", TaxID: tid},
			[]*Check{validCheck(&Record{Name: "  ACME GMBH \n"})}, `{"name":"ACME GMBH"}`},
		{"trimmed name equal to the party's writes nothing", &org.Party{Name: "ACME GMBH", TaxID: tid},
			[]*Check{validCheck(&Record{Name: " ACME GMBH "})}, `{}`},
		{"empty structured address writes nothing", &org.Party{TaxID: tid},
			[]*Check{validCheck(&Record{Address: &org.Address{}})}, `{}`},
		{"label-only address writes nothing: a label does not locate", &org.Party{TaxID: tid, Addresses: []*org.Address{billing}},
			[]*Check{validCheck(&Record{Address: &org.Address{Label: "Registered Office"}})}, `{}`},
		{"same place under another label writes nothing", &org.Party{TaxID: tid, Addresses: []*org.Address{{Label: "HQ", Street: "Hauptstrasse", Number: "1", Code: "10115", Locality: "Berlin", Country: "DE"}}},
			[]*Check{validCheck(&Record{Address: office})}, `{}`},
		{"nil first address is replaced, nil others are dropped", &org.Party{TaxID: tid, Addresses: []*org.Address{nil, billing, nil}},
			[]*Check{validCheck(&Record{Address: office})},
			`{"addresses":[{"num":"1","street":"Hauptstrasse","locality":"Berlin","code":"10115","country":"DE"},{"label":"Billing","locality":"Munich","country":"DE"}]}`},
		{"registered address already in the party moves first, once", &org.Party{TaxID: tid, Addresses: []*org.Address{billing, office}},
			[]*Check{validCheck(&Record{Address: office})},
			`{"addresses":[{"num":"1","street":"Hauptstrasse","locality":"Berlin","code":"10115","country":"DE"},{"label":"Billing","locality":"Munich","country":"DE"}]}`},
		{"nil check in the report is skipped", &org.Party{TaxID: tid},
			[]*Check{nil, validCheck(&Record{Name: "ACME"})}, `{"name":"ACME"}`},
		{"empty report writes nothing", &org.Party{TaxID: tid}, nil, `{}`},
	}
	for _, tt := range edge {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := patchOf(tt.party, tt.checks...)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(patch))
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := patchOf(tt.party, tt.checks...)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(patch))
		})
	}

	t.Run("nil input", func(t *testing.T) {
		_, err := Patch(nil, &Report{})
		assert.ErrorIs(t, err, ErrInput)
		_, err = Patch(&org.Party{}, nil)
		assert.ErrorIs(t, err, ErrInput)
	})

	t.Run("first valid record wins", func(t *testing.T) {
		patch, err := patchOf(&org.Party{TaxID: tid},
			&Check{Status: StatusUnverified},
			validCheck(&Record{Name: "FIRST"}),
			validCheck(&Record{Name: "SECOND"}))
		require.NoError(t, err)
		assert.JSONEq(t, `{"name":"FIRST"}`, string(patch))
	})

	t.Run("applied, it keeps the tax ID", func(t *testing.T) {
		party := &org.Party{Name: "Acme", TaxID: tid, Addresses: []*org.Address{billing}}
		patch, err := patchOf(party, validCheck(&Record{Name: "ACME TRADING GMBH", Address: office}))
		require.NoError(t, err)
		got := applyPatch(t, party, patch)
		assert.Equal(t, tid, got.TaxID)
		assert.Equal(t, "ACME TRADING GMBH", got.Name)
		assert.Equal(t, []*org.Address{office}, got.Addresses, "the first address is replaced, a single one included")
	})
}

// TestPatchSkipsAnEchoedOtherTaxID checks a valid check whose register
// echoed a different tax ID writes nothing: its record describes another
// party.
func TestPatchSkipsAnEchoedOtherTaxID(t *testing.T) {
	party := &org.Party{Name: "Acme", TaxID: &tax.Identity{Country: "DE", Code: "282741168"}}
	check := validCheck(&Record{Name: "OTHER GMBH"})
	check.Mismatches = []*Mismatch{{Field: MismatchTaxID, Path: "/tax_id/code", Document: "282741168", Register: "111111125"}}

	patch, err := patchOf(party, check)
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(patch))
}
