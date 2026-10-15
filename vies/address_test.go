package vies

import (
	"context"
	"net/http"
	"testing"

	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseAddressIT uses addresses of companies as VIES gave them on
// 2026-10-15, plus made-up lines for the shapes the samples did not show.
func TestParseAddressIT(t *testing.T) {
	it := func(street, num, extra, code, locality, region string) *org.Address {
		return &org.Address{Street: street, Number: num, StreetExtra: extra, Code: cbc.Code(code), Locality: locality, Region: region, Country: "IT"}
	}
	tests := []struct {
		name string
		raw  string
		want *org.Address
	}{
		{"street and number", "VIA EMILIA EST 1163 \n41122 MODENA MO\n", it("VIA EMILIA EST", "1163", "", "41122", "MODENA", "MO")},
		{"piazzale", "PIAZZALE ENRICO MATTEI 1 \n00144 ROMA RM\n", it("PIAZZALE ENRICO MATTEI", "1", "", "00144", "ROMA", "RM")},
		{"N before the number", "VIALE PIERO E ALBERTO PIRELLI N 25 \n20126 MILANO MI\n", it("VIALE PIERO E ALBERTO PIRELLI", "25", "", "20126", "MILANO", "MI")},
		{"number range", "VIA EUGENIO CHECCHI 17-19 \n00157 ROMA RM\n", it("VIA EUGENIO CHECCHI", "17-19", "", "00157", "ROMA", "RM")},
		{"number with a letter", "VIA DEI CERASETI 23A \n00047 MARINO RM\n", it("VIA DEI CERASETI", "23A", "", "00047", "MARINO", "RM")},
		{"flat after the number", "VIA DEL BOSCO DEGLI ARVALI 84 INT 7\n00148 ROMA RM\n", it("VIA DEL BOSCO DEGLI ARVALI", "84", "INT 7", "00148", "ROMA", "RM")},
		{"staircase and flat", "VIALE AMELIA 32 SC UN INT 14 \n00181 ROMA RM\n", it("VIALE AMELIA", "32", "SC UN INT 14", "00181", "ROMA", "RM")},
		{"N and flat", "VIA TRIONFALE N 85 INT 3 \n00136 ROMA RM\n", it("VIA TRIONFALE", "85", "INT 3", "00136", "ROMA", "RM")},
		{"number in the street name", "VIA 4 NOVEMBRE 10\n00187 ROMA RM", it("VIA 4 NOVEMBRE", "10", "", "00187", "ROMA", "RM")},
		{"no house number", "VIA 4 NOVEMBRE\n00187 ROMA RM", it("VIA 4 NOVEMBRE", "", "", "00187", "ROMA", "RM")},
		{"locality of several words", "VIA ROMA 1\n20090 SEGRATE MILANO MI", it("VIA ROMA", "1", "", "20090", "SEGRATE MILANO", "MI")},
		{"masked", "---", nil},
		{"empty", "", nil},
		{"one line", "VIA ROMA 1 00187 ROMA RM", nil},
		{"three lines", "C/O STUDIO ROSSI\nVIA ROMA 1\n00187 ROMA RM", nil},
		{"no province", "VIA ROMA 1\n00187 ROMA", nil},
		{"short postcode", "VIA ROMA 1\n0187 ROMA RM", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseAddress("IT", tt.raw))
		})
	}
}

func TestParseAddressWithoutParser(t *testing.T) {
	assert.Nil(t, parseAddress("DE", "MUSTERSTR. 1\n10115 BERLIN"), "a member state without a parser gives no address")
	assert.Nil(t, parseAddress("FR", "59 RUE LA FAYETTE\n75009 PARIS"), "France has no parser")
}

func TestVerifyAddress(t *testing.T) {
	t.Run("an Italian address is structured", func(t *testing.T) {
		v := serve(t, http.StatusOK, `{"countryCode":"IT","vatNumber":"00159560366","valid":true,"name":"FERRARI S.P.A.","address":"VIA EMILIA EST 1163 \n41122 MODENA MO\n"}`, nil)
		ans, err := v.Verify(context.Background(), tin.Request{Identifier: tin.Identifier{Path: tin.PathTaxID, Country: "IT", Code: "00159560366"}})
		require.NoError(t, err)
		require.NotNil(t, ans.Record)
		assert.Equal(t, "FERRARI S.P.A.", ans.Record.Name)
		assert.Equal(t, &org.Address{Street: "VIA EMILIA EST", Number: "1163", Code: "41122", Locality: "MODENA", Region: "MO", Country: "IT"}, ans.Record.Address)
	})

	t.Run("an address of an unknown shape is dropped and the name kept", func(t *testing.T) {
		v := serve(t, http.StatusOK, `{"countryCode":"IT","vatNumber":"00159560366","valid":true,"name":"FERRARI S.P.A.","address":"VIA EMILIA EST 1163 MODENA"}`, nil)
		ans, err := v.Verify(context.Background(), tin.Request{Identifier: tin.Identifier{Path: tin.PathTaxID, Country: "IT", Code: "00159560366"}})
		require.NoError(t, err)
		assert.Equal(t, &tin.Record{Name: "FERRARI S.P.A."}, ans.Record)
	})
}
