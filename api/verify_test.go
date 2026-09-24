package api

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerificationValid(t *testing.T) {
	tests := []struct {
		name   string
		checks []*Check
		want   bool
	}{
		{name: "nil report", checks: nil, want: false},
		{name: "empty report", checks: []*Check{}, want: false},
		{name: "one valid", checks: []*Check{{Status: StatusValid}}, want: true},
		{name: "valid and unsupported", checks: []*Check{{Status: StatusValid}, {Status: StatusUnsupported}}, want: true},
		{name: "only unsupported", checks: []*Check{{Status: StatusUnsupported}}, want: false},
		{name: "one invalid", checks: []*Check{{Status: StatusValid}, {Status: StatusInvalid}}, want: false},
		{name: "one unverified", checks: []*Check{{Status: StatusValid}, {Status: StatusUnverified}}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Verification{Checks: tt.checks}
			assert.Equal(t, tt.want, v.Valid())
		})
	}

	t.Run("nil pointer", func(t *testing.T) {
		var v *Verification
		assert.False(t, v.Valid())
	})
}

func TestCheckFail(t *testing.T) {
	cause := ErrServer.WithCode("500").WithMessage("MS_UNAVAILABLE")
	c := &Check{Path: PathTaxID, Status: StatusValid}
	c.Fail(cause)
	assert.Equal(t, StatusUnverified, c.Status)
	assert.Equal(t, "server: 500: MS_UNAVAILABLE", c.Error)
	assert.True(t, errors.Is(c.Err(), ErrServer))

	c.Fail(nil)
	assert.Equal(t, StatusUnverified, c.Status)
	assert.Empty(t, c.Error)
	assert.NoError(t, c.Err())
}

func TestCheckJSON(t *testing.T) {
	when := time.Date(2026, 9, 24, 9, 12, 0, 0, time.UTC)
	v := &Verification{Checks: []*Check{
		{
			Path:      PathTaxID,
			TaxID:     &tax.Identity{Country: "DE", Code: "282741168"},
			Status:    StatusValid,
			Source:    "vies",
			CheckedAt: when,
			Record:    &Record{Name: "ACME TRADING GMBH"},
			Mismatches: []*Mismatch{
				{Path: "name", Document: "Acme Trading", Register: "ACME TRADING GMBH"},
			},
		},
		{Path: "identities[0]", Status: StatusUnsupported},
	}}
	data, err := json.Marshal(v)
	require.NoError(t, err)
	want := `{"checks":[` +
		`{"path":"tax_id","tax_id":{"country":"DE","code":"282741168"},"status":"valid","source":"vies","checked_at":"2026-09-24T09:12:00Z","record":{"name":"ACME TRADING GMBH"},"mismatches":[{"path":"name","document":"Acme Trading","register":"ACME TRADING GMBH"}]},` +
		`{"path":"identities[0]","status":"unsupported"}` +
		`]}`
	assert.JSONEq(t, want, string(data))

	t.Run("unverified carries only the message", func(t *testing.T) {
		c := &Check{Path: PathTaxID, Source: "vies"}
		c.Fail(ErrNetwork.WithMessage("dial"))
		data, err := json.Marshal(c)
		require.NoError(t, err)
		assert.JSONEq(t, `{"path":"tax_id","status":"unverified","source":"vies","error":"network: dial"}`, string(data))
	})
}
