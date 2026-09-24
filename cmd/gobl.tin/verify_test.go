package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/invopop/gobl.tin/api"
	"github.com/invopop/gobl/cbc"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeVIES stands in for the VIES verifier: same source, canned answers by
// code, so that the command never dials the register.
type fakeVIES struct {
	checks map[cbc.Code]*api.Check
	errs   map[cbc.Code]error
}

func (f *fakeVIES) Source() cbc.Key { return "vies" }

func (f *fakeVIES) Supports(id api.Identifier) bool {
	return id.Type == "" && id.Key == ""
}

func (f *fakeVIES) Verify(_ context.Context, id api.Identifier) (*api.Check, error) {
	if err, ok := f.errs[id.Code]; ok {
		return nil, err
	}
	if c, ok := f.checks[id.Code]; ok {
		cp := *c
		return &cp, nil
	}
	return &api.Check{Status: api.StatusInvalid, Source: "vies", CheckedAt: time.Now()}, nil
}

var checkedAt = time.Date(2026, 9, 24, 9, 12, 0, 0, time.UTC)

func validCheck(name string) *api.Check {
	c := &api.Check{Status: api.StatusValid, Source: "vies", CheckedAt: checkedAt}
	if name != "" {
		c.Record = &api.Record{Name: name}
	}
	return c
}

// runVerify executes the verify command with the fake and returns stdout and the
// error.
func runVerify(t *testing.T, fake *fakeVIES, args ...string) (string, error) {
	t.Helper()
	cmd := &cobra.Command{SilenceUsage: true, SilenceErrors: true}
	vo := verify(&rootOpts{})
	if fake != nil {
		vo.verifiers = []api.Verifier{fake}
	}
	cmd.AddCommand(vo.cmd())
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs(append([]string{"verify"}, args...))
	err := cmd.Execute()
	return out.String(), err
}

func TestVerifyCommand(t *testing.T) {
	// The customer of invoice-valid.json is 282741168 and the supplier is
	// 111111125; party.json carries 282741168 and an HRB identity.
	allValid := &fakeVIES{checks: map[cbc.Code]*api.Check{
		"282741168": validCheck("ACME TRADING GMBH"),
		"111111125": validCheck(""),
	}}

	t.Run("party document", func(t *testing.T) {
		out, err := runVerify(t, allValid, "../../test/data/party.json")
		require.NoError(t, err)
		assert.Equal(t, "/tax_id: valid (vies)\n"+
			"  /name: document \"Acme Trading\", register \"ACME TRADING GMBH\"\n"+
			"/identities/0: unsupported\n", out)
	})

	t.Run("invoice customer by default", func(t *testing.T) {
		out, err := runVerify(t, allValid, "../../test/data/invoice-valid.json")
		require.NoError(t, err)
		assert.Equal(t, "/tax_id: valid (vies)\n"+
			"  /name: document \"Sample Consumer\", register \"ACME TRADING GMBH\"\n", out)
	})

	t.Run("invoice supplier", func(t *testing.T) {
		out, err := runVerify(t, allValid, "../../test/data/invoice-valid.json", "--party", "supplier")
		require.NoError(t, err)
		assert.Equal(t, "/tax_id: valid (vies)\n", out)
	})

	t.Run("invoice both parties are labelled", func(t *testing.T) {
		out, err := runVerify(t, allValid, "../../test/data/invoice-valid.json", "--party", "both")
		require.NoError(t, err)
		assert.Equal(t, "customer:\n"+
			"  /tax_id: valid (vies)\n"+
			"    /name: document \"Sample Consumer\", register \"ACME TRADING GMBH\"\n"+
			"supplier:\n"+
			"  /tax_id: valid (vies)\n", out)
	})

	t.Run("invalid check exits non-zero after printing", func(t *testing.T) {
		fake := &fakeVIES{checks: map[cbc.Code]*api.Check{"111111125": validCheck("")}}
		out, err := runVerify(t, fake, "../../test/data/invoice-valid.json", "--party", "both")
		assert.ErrorIs(t, err, errNotValid)
		assert.Equal(t, "customer:\n"+
			"  /tax_id: invalid (vies)\n"+
			"supplier:\n"+
			"  /tax_id: valid (vies)\n", out)
	})

	t.Run("unverified check prints the failure and exits non-zero", func(t *testing.T) {
		fake := &fakeVIES{errs: map[cbc.Code]error{"282741168": api.ErrServer.WithCode("500").WithMessage("MS_UNAVAILABLE")}}
		out, err := runVerify(t, fake, "../../test/data/party.json")
		assert.ErrorIs(t, err, errNotValid)
		assert.Equal(t, "/tax_id: unverified (vies): server: 500: MS_UNAVAILABLE\n"+
			"/identities/0: unsupported\n", out)
	})

	t.Run("json prints the report", func(t *testing.T) {
		out, err := runVerify(t, allValid, "../../test/data/party.json", "--json")
		require.NoError(t, err)
		assert.JSONEq(t, `{"checks":[`+
			`{"path":"/tax_id","tax_id":{"country":"DE","code":"282741168"},"status":"valid","source":"vies","checked_at":"2026-09-24T09:12:00Z","record":{"name":"ACME TRADING GMBH"},"mismatches":[{"field":"name","path":"/name","document":"Acme Trading","register":"ACME TRADING GMBH"}]},`+
			`{"path":"/identities/0","identity":{"country":"DE","type":"HRB","code":"12345"},"status":"unsupported"}`+
			`]}`, out)
	})

	t.Run("json with both parties is keyed by label", func(t *testing.T) {
		out, err := runVerify(t, allValid, "../../test/data/invoice-valid.json", "--party", "both", "--json")
		require.NoError(t, err)
		var got map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(out), &got))
		assert.Len(t, got, 2)
		assert.Contains(t, got, "customer")
		assert.Contains(t, got, "supplier")
	})

	t.Run("missing customer", func(t *testing.T) {
		_, err := runVerify(t, allValid, "../../test/data/invoice-no-customer.json")
		assert.EqualError(t, err, "invoice has no customer")
	})

	t.Run("unknown party selector", func(t *testing.T) {
		_, err := runVerify(t, allValid, "../../test/data/invoice-valid.json", "--party", "everyone")
		assert.EqualError(t, err, `invalid party "everyone", expected customer, supplier or both`)
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := runVerify(t, allValid, "../../test/data/does-not-exist.json")
		assert.Error(t, err)
	})

	t.Run("argument count", func(t *testing.T) {
		_, err := runVerify(t, allValid)
		assert.Error(t, err)
		_, err = runVerify(t, allValid, "a", "b")
		assert.Error(t, err)
	})
}
