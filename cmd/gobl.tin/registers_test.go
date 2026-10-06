package main

import (
	"errors"
	"testing"

	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl.tin/aeat"
	"github.com/invopop/gobl.tin/vies"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRegister returns a fixed verifier or error.
type stubRegister struct {
	v   tin.Verifier
	err error
}

func (*stubRegister) flags(*pflag.FlagSet) {}

func (s *stubRegister) verifier() (tin.Verifier, error) { return s.v, s.err }

func TestBuildVerifiers(t *testing.T) {
	t.Run("keeps the order and skips registers that are not configured", func(t *testing.T) {
		a, b := &fakeVIES{}, &fakeVIES{}
		vs, err := buildVerifiers([]register{&stubRegister{v: a}, &stubRegister{}, &stubRegister{v: b}})
		require.NoError(t, err)
		require.Len(t, vs, 2)
		assert.Same(t, a, vs[0])
		assert.Same(t, b, vs[1])
	})

	t.Run("stops at the first error", func(t *testing.T) {
		boom := errors.New("boom")
		_, err := buildVerifiers([]register{&stubRegister{err: boom}, &stubRegister{v: &fakeVIES{}}})
		assert.ErrorIs(t, err, boom)
	})
}

func TestDefaultRegisters(t *testing.T) {
	t.Run("VIES alone without an AEAT certificate", func(t *testing.T) {
		vs, err := buildVerifiers(defaultRegisters())
		require.NoError(t, err)
		require.Len(t, vs, 1)
		assert.Equal(t, vies.Source, vs[0].Source())
	})

	t.Run("AEAT before VIES with a certificate", func(t *testing.T) {
		t.Setenv(envAEATPassword, "secret")
		path := writeCertificate(t, "secret")
		vo := verify(&rootOpts{})
		require.NoError(t, vo.cmd().Flags().Parse([]string{"--aeat-cert", path}))
		vs, err := buildVerifiers(vo.registers)
		require.NoError(t, err)
		require.Len(t, vs, 2)
		assert.Equal(t, aeat.Source, vs[0].Source())
		assert.Equal(t, vies.Source, vs[1].Source())
	})
}
