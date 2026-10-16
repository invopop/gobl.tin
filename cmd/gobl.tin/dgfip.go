package main

import (
	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl.tin/dgfip"
	"github.com/spf13/pflag"
)

// dgfipRegister wires the DGFiP. It needs no configuration, so it always
// runs, before VIES, so that it answers for French tax IDs.
type dgfipRegister struct{}

func (*dgfipRegister) flags(*pflag.FlagSet) {}

func (*dgfipRegister) verifier() (tin.Verifier, error) {
	return dgfip.New(), nil
}
