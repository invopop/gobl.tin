package main

import (
	"testing"

	"github.com/invopop/gobl.tin/hmrc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHMRCRegister(t *testing.T) {
	t.Run("no client ID is not configured", func(t *testing.T) {
		v, err := (&hmrcRegister{}).verifier()
		require.NoError(t, err)
		assert.Nil(t, v)
	})

	t.Run("a client ID builds the HMRC verifier", func(t *testing.T) {
		t.Setenv(envHMRCSecret, "secret")
		v, err := (&hmrcRegister{clientID: "id", sandbox: true}).verifier()
		require.NoError(t, err)
		require.NotNil(t, v)
		assert.Equal(t, hmrc.Source, v.Source())
	})
}
