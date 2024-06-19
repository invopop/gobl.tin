package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/invopop/gobl"
	gobltin "github.com/invopop/gobl.tin"
	"github.com/spf13/cobra"
)

type lookupOpts struct {
	*rootOpts
}

func lookup(o *rootOpts) *lookupOpts {
	return &lookupOpts{rootOpts: o}
}

func (c *lookupOpts) cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lookup <input>",
		Short: "Check validity for the customer TIN number in an invoice",
		RunE:  c.runE,
	}

	return cmd
}

func (c *lookupOpts) runE(cmd *cobra.Command, args []string) error {

	if len(args) != 1 {
		return fmt.Errorf("expected exactly one input file, the command usage is `gobl.tin lookup <input>`")
	}

	input, err := openInput(cmd, args)
	if err != nil {
		return err
	}
	defer input.Close()

	inData, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	env := new(gobl.Envelope)
	if err := json.Unmarshal(inData, env); err != nil {
		return fmt.Errorf("parsing input as GOBL Envelope: %w", err)
	}

	tin, err := gobltin.NewTinNumber(env)
	if err != nil {
		return fmt.Errorf("creating TIN number: %w", err)
	}

	response, err := gobltin.Lookup(cmd.Context(), tin)
	if err != nil {
		return fmt.Errorf("looking up TIN number: %w", err)
	}

	if response.Valid {
		fmt.Println("TIN number is valid")
	} else {
		fmt.Println("TIN number is invalid")
	}
	return nil
}
