package tin

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportValid(t *testing.T) {
	tests := []struct {
		name   string
		checks []*Check
		want   bool
	}{
		{name: "nil report", checks: nil, want: false},
		{name: "empty report", checks: []*Check{}, want: false},
		{name: "one valid", checks: []*Check{{Status: StatusValid}}, want: true},
		{name: "nil checks are skipped", checks: []*Check{nil, {Status: StatusValid}}, want: true},
		{name: "valid and unsupported", checks: []*Check{{Status: StatusValid}, {Status: StatusUnsupported}}, want: true},
		{name: "only unsupported", checks: []*Check{{Status: StatusUnsupported}}, want: false},
		{name: "one invalid", checks: []*Check{{Status: StatusValid}, {Status: StatusInvalid}}, want: false},
		{name: "one unverified", checks: []*Check{{Status: StatusValid}, {Status: StatusUnverified}}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Report{Checks: tt.checks}
			assert.Equal(t, tt.want, v.Valid())
		})
	}

	t.Run("nil pointer", func(t *testing.T) {
		var v *Report
		assert.False(t, v.Valid())
	})
}

func TestCheckFailure(t *testing.T) {
	cause := ErrServer.WithCode("500").WithMessage("MS_UNAVAILABLE")
	c := &Check{Path: PathTaxID, Status: StatusValid}
	c.fail(cause)
	assert.Equal(t, StatusUnverified, c.Status)
	assert.Equal(t, "server: 500: MS_UNAVAILABLE", c.Failure)
	assert.True(t, errors.Is(c.Err(), ErrServer))

	c.fail(nil)
	assert.Equal(t, StatusUnverified, c.Status)
	assert.Empty(t, c.Failure)
	assert.NoError(t, c.Err())
}

func TestCheckJSON(t *testing.T) {
	when := time.Date(2026, 9, 24, 9, 12, 0, 0, time.UTC)
	v := &Report{Checks: []*Check{
		{
			Path:      PathTaxID,
			TaxID:     &tax.Identity{Country: "DE", Code: "282741168"},
			Status:    StatusValid,
			Source:    "vies",
			CheckedAt: when,
			Record:    &Record{Name: "ACME TRADING GMBH"},
			Mismatches: []*Mismatch{
				{Field: MismatchName, Path: "/name", Document: "Acme Trading", Register: "ACME TRADING GMBH"},
			},
		},
		{Path: "/identities/0", Status: StatusUnsupported},
	}}
	data, err := json.Marshal(v)
	require.NoError(t, err)
	want := `{"checks":[` +
		`{"path":"/tax_id","tax_id":{"country":"DE","code":"282741168"},"status":"valid","source":"vies","checked_at":"2026-09-24T09:12:00Z","record":{"name":"ACME TRADING GMBH"},"mismatches":[{"field":"name","path":"/name","document":"Acme Trading","register":"ACME TRADING GMBH"}]},` +
		`{"path":"/identities/0","status":"unsupported"}` +
		`]}`
	assert.JSONEq(t, want, string(data))

	t.Run("unverified carries only the message", func(t *testing.T) {
		c := &Check{Path: PathTaxID, Source: "vies"}
		c.fail(ErrNetwork.WithMessage("dial"))
		data, err := json.Marshal(c)
		require.NoError(t, err)
		assert.JSONEq(t, `{"path":"/tax_id","status":"unverified","source":"vies","failure":"network: dial"}`, string(data))
	})
}
