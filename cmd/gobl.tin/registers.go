package main

import (
	tin "github.com/invopop/gobl.tin"
	"github.com/spf13/pflag"
)

// register wires one verifier into the command: the flags it needs and how
// to build it from them. Each register keeps its specifics in its own file,
// so that the verify command knows none of them.
type register interface {
	// flags adds the register's flags to the command.
	flags(fs *pflag.FlagSet)

	// verifier builds the verifier from the flags. It returns nil when
	// the register is not configured, so that the command skips it.
	verifier() (tin.Verifier, error)
}

// defaultRegisters lists the registers in routing order: a national register
// comes before VIES, so that it answers for its own country.
func defaultRegisters() []register {
	return []register{new(aeatRegister), new(viesRegister)}
}

// buildVerifiers builds the verifiers of the configured registers, in order.
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
