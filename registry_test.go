package tin

import (
	"testing"

	"github.com/invopop/gobl/l10n"
	"github.com/stretchr/testify/assert"
)

func TestSupported(t *testing.T) {
	assert.True(t, Supported("ES"))
	assert.True(t, Supported("EL"), "VIES uses EL for Greece")
	assert.True(t, Supported("XI"), "VIES covers Northern Ireland as XI")
	assert.False(t, Supported("US"))
	assert.False(t, Supported("GB"), "GB proper is not covered by VIES")
	assert.False(t, Supported(""))
}

func TestCountries(t *testing.T) {
	countries := Countries()
	assert.Len(t, countries, 28)
	assert.Contains(t, countries, l10n.TaxCountryCode("ES"))
	assert.Contains(t, countries, l10n.TaxCountryCode("XI"))

	// Every listed country reports as supported, and mutating the returned
	// slice does not affect the routing table.
	for _, c := range countries {
		assert.True(t, Supported(c), string(c))
	}
	countries[0] = "US"
	assert.True(t, Supported(Countries()[0]))
}
