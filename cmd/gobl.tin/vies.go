package main

import (
	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl.tin/vies"
	"github.com/spf13/pflag"
)

// viesRegister wires VIES. It needs no configuration, so it always runs.
type viesRegister struct{}

func (*viesRegister) flags(*pflag.FlagSet) {}

func (*viesRegister) verifier() (tin.Verifier, error) {
	return vies.New(), nil
}
