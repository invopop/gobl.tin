package main

import (
	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl.tin/vies"
	"github.com/spf13/pflag"
)

// Wires VIES, with HMRC records for Northern Ireland when HMRC is configured.
type viesRegister struct {
	northernIreland *hmrcRegister
}

func (*viesRegister) flags(*pflag.FlagSet) {}

func (r *viesRegister) verifier() (tin.Verifier, error) {
	var opts []vies.Option
	if r.northernIreland != nil {
		uk, err := r.northernIreland.verifier()
		if err != nil {
			return nil, err
		}
		if uk != nil {
			opts = append(opts, vies.WithNorthernIrelandRecords(uk))
		}
	}
	return vies.New(opts...), nil
}
