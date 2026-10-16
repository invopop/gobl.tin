package main

import (
	tin "github.com/invopop/gobl.tin"
	"github.com/spf13/pflag"
)

// Wires one verifier into the command.
type register interface {
	flags(fs *pflag.FlagSet)

	// Nil when the register is not configured.
	verifier() (tin.Verifier, error)
}

// Registers in routing order: national registers before VIES.
func defaultRegisters() []register {
	return []register{new(aeatRegister), new(dgfipRegister), new(viesRegister)}
}

func buildVerifiers(registers []register) ([]tin.Verifier, error) {
	var out []tin.Verifier
	for _, r := range registers {
		v, err := r.verifier()
		if err != nil {
			return nil, err
		}
		if v != nil {
			out = append(out, v)
		}
	}
	return out, nil
}
