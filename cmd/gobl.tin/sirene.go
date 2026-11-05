package main

import (
	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl.tin/sirene"
	"github.com/spf13/pflag"
)

// sireneRegister wires Sirene. It needs no configuration, so it always runs
// and checks the SIREN identities of a party.
type sireneRegister struct{}

func (*sireneRegister) flags(*pflag.FlagSet) {}

func (*sireneRegister) verifier() (tin.Verifier, error) {
	return sirene.New(), nil
}
