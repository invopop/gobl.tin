package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/invopop/gobl"
	tin "github.com/invopop/gobl.tin"
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

	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return fmt.Errorf("invalid type %T", env.Document)
	}

	ctx := context.Background()
	client := tin.New()

	switch c.lookupType {
	case "customer":
		err := client.Lookup(ctx, inv.Customer)
		if err != nil {
			var e *tin.Error
			if errors.As(err, &e) {
				if e.Is(tin.ErrInvalid) {
					if _, err := fmt.Fprint(cmd.OutOrStdout(), e.Error()); err != nil {
						return fmt.Errorf("writing output: %w", err)
					}
					return nil
				}
				return fmt.Errorf("looking up customer TIN number: %w", err)
			}
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Customer: TIN is valid\n"); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	case "supplier":
		err := client.Lookup(ctx, inv.Supplier)
		if err != nil {
			var e *tin.Error
			if errors.As(err, &e) {
				if e.Is(tin.ErrInvalid) {
					if _, err := fmt.Fprint(cmd.OutOrStdout(), e.Error()); err != nil {
						return fmt.Errorf("writing output: %w", err)
					}
					return nil
				}
				return fmt.Errorf("looking up supplier TIN number: %w", err)
			}
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Supplier: TIN is valid"); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	case "both":
		err := client.Lookup(ctx, inv)
		if err != nil {
			var e *tin.Error
			if errors.As(err, &e) {
				if e.Is(tin.ErrInvalid) {
					if _, err := fmt.Fprint(cmd.OutOrStdout(), e.Error()); err != nil {
						return fmt.Errorf("writing output: %w", err)
					}
					return nil
				}
				return fmt.Errorf("looking up TIN number: %w", err)
			}
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Customer: TIN is valid\nSupplier: Tax ID is valid"); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	default:
		return fmt.Errorf("invalid lookup type: %s, expected customer, supplier, or both", c.lookupType)
	}

	return nil
}
