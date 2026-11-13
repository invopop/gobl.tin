package vies

import (
	"context"
	"errors"
	"net/http"
	"testing"

	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRecords stands in for HMRC as the record source of XI numbers.
type fakeRecords struct {
	ans  *tin.Answer
	err  error
	seen []tin.Identifier
}

func (f *fakeRecords) Source() cbc.Key { return "hmrc" }
func (f *fakeRecords) Supports(id tin.Identifier) bool {
	return id.Country == "GB"
}
func (f *fakeRecords) Verify(_ context.Context, req tin.Request) (*tin.Answer, error) {
	f.seen = append(f.seen, req.Identifier)
	return f.ans, f.err
}

// tescoXI is the VIES answer for Tesco's XI number on 2026-10-15.
const tescoXI = `{"countryCode":"XI","vatNumber":"220430231","valid":true,"name":"TESCO PLC","address":"CIRRUS A SHIRE PARK KESTREL WAY WELWYN GARDEN CITY   AL7 1GA"}`

func xiRequest() tin.Request {
	return tin.Request{Identifier: tin.Identifier{Path: tin.PathTaxID, Country: "XI", Code: "220430231"}}
}

func TestNorthernIrelandRecords(t *testing.T) {
	hmrcRecord := &tin.Record{
		Name:    "TESCO PLC",
		Address: &org.Address{Street: "CIRRUS A SHIRE PARK", StreetExtra: "KESTREL WAY", Locality: "WELWYN GARDEN CITY", Code: "AL7 1GA", Country: "GB"},
	}

	t.Run("a valid XI number takes the record of the GB number", func(t *testing.T) {
		src := &fakeRecords{ans: &tin.Answer{Status: tin.StatusValid, Record: hmrcRecord}}
		v := serve(t, http.StatusOK, tescoXI, nil)
		WithNorthernIrelandRecords(src)(v)
		ans, err := v.Verify(context.Background(), xiRequest())
		require.NoError(t, err)
		assert.Equal(t, tin.StatusValid, ans.Status)
		assert.Equal(t, hmrcRecord, ans.Record)
		assert.Equal(t, "XI", ans.TaxID.Country.String(), "the checked number stays the XI number")
		require.Len(t, src.seen, 1)
		assert.Equal(t, tin.Identifier{Path: tin.PathTaxID, Country: "GB", Code: "220430231"}, src.seen[0])
	})

	t.Run("a failing record source keeps the VIES record", func(t *testing.T) {
		src := &fakeRecords{err: errors.New("hmrc down")}
		v := serve(t, http.StatusOK, tescoXI, nil)
		WithNorthernIrelandRecords(src)(v)
		ans, err := v.Verify(context.Background(), xiRequest())
		require.NoError(t, err)
		assert.Equal(t, &tin.Record{Name: "TESCO PLC"}, ans.Record)
	})

	t.Run("a GB number that the source does not hold keeps the VIES record", func(t *testing.T) {
		src := &fakeRecords{ans: &tin.Answer{Status: tin.StatusInvalid}}
		v := serve(t, http.StatusOK, tescoXI, nil)
		WithNorthernIrelandRecords(src)(v)
		ans, err := v.Verify(context.Background(), xiRequest())
		require.NoError(t, err)
		assert.Equal(t, tin.StatusValid, ans.Status, "VIES decides validity")
		assert.Equal(t, &tin.Record{Name: "TESCO PLC"}, ans.Record)
	})

	t.Run("an invalid XI number does not ask the source", func(t *testing.T) {
		src := &fakeRecords{ans: &tin.Answer{Status: tin.StatusValid, Record: hmrcRecord}}
		v := serve(t, http.StatusOK, `{"countryCode":"XI","vatNumber":"220430231","valid":false,"name":"---","address":"---"}`, nil)
		WithNorthernIrelandRecords(src)(v)
		ans, err := v.Verify(context.Background(), xiRequest())
		require.NoError(t, err)
		assert.Equal(t, tin.StatusInvalid, ans.Status)
		assert.Nil(t, ans.Record)
		assert.Empty(t, src.seen)
	})

	t.Run("other countries do not ask the source", func(t *testing.T) {
		src := &fakeRecords{ans: &tin.Answer{Status: tin.StatusValid, Record: hmrcRecord}}
		v := serve(t, http.StatusOK, viesValidNamedBody, nil)
		WithNorthernIrelandRecords(src)(v)
		_, err := v.Verify(context.Background(), tin.Request{Identifier: tin.Identifier{Path: tin.PathTaxID, Country: "DE", Code: "282741168"}})
		require.NoError(t, err)
		assert.Empty(t, src.seen)
	})

	t.Run("without a source the VIES record stays", func(t *testing.T) {
		v := serve(t, http.StatusOK, tescoXI, nil)
		ans, err := v.Verify(context.Background(), xiRequest())
		require.NoError(t, err)
		assert.Equal(t, &tin.Record{Name: "TESCO PLC"}, ans.Record, "VIES gives no XI address")
	})
}
