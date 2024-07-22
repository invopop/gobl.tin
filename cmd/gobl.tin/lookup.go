package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/invopop/gobl"
	gobltin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl/bill"
	"github.com/spf13/cobra"
)

type lookupOpts struct {
	*rootOpts
	lookupType string
}

func lookup(o *rootOpts) *lookupOpts {
	return &lookupOpts{rootOpts: o}
}

func (c *lookupOpts) cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lookup <input>",
		Short: "Check validity for the customer and/or the supplier TIN number in an invoice",
		RunE:  c.runE,
	}

	cmd.Flags().StringVarP(&c.lookupType, "type", "t", "customer", "Type of lookup: customer, supplier, or both")

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
	defer func() {
		if cerr := input.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	inData, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	env := new(gobl.Envelope)
	if err := json.Unmarshal(inData, env); err != nil {
		return fmt.Errorf("parsing input as GOBL Envelope: %w", err)
	}

	var valid bool

	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return fmt.Errorf("invalid type %T", env.Document)
	}

	ctx := cmd.Context()

	switch c.lookupType {
	case "customer":
		valid, err = gobltin.LookupTin(ctx, inv.Customer)
		if err != nil {
			return fmt.Errorf("looking up customer TIN number: %w", err)
		}
		if err := c.displayOutput("Customer", valid, cmd); err != nil {
			return err
		}
	case "supplier":
		valid, err = gobltin.LookupTin(ctx, inv.Supplier)
		if err != nil {
			return fmt.Errorf("looking up supplier TIN number: %w", err)
		}
		if err := c.displayOutput("Supplier", valid, cmd); err != nil {
			return err
		}
	case "both":
		valid, err = gobltin.LookupTin(ctx, inv.Customer)
		if err != nil {
			return fmt.Errorf("looking up customer TIN number: %w", err)
		}
		if err := c.displayOutput("Customer", valid, cmd); err != nil {
			return err
		}

		valid, err = gobltin.LookupTin(ctx, inv.Supplier)
		if err != nil {
			return fmt.Errorf("looking up supplier TIN number: %w", err)
		}
		if err := c.displayOutput("Supplier", valid, cmd); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid lookup type: %s, expected customer, supplier, or both", c.lookupType)
	}

	return nil
}

func (c *lookupOpts) displayOutput(party string, valid bool, cmd *cobra.Command) error {
	if valid {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s Tax ID is VALID\n", party); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	} else {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s Tax ID is INVALID\n", party); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	}
	return nil
}
