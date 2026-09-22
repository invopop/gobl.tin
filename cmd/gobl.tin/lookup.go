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

// errInvalidTIN makes the command exit non-zero when a lookup answers that a
// TIN is invalid. The per-party status lines are already on stdout by then.
var errInvalidTIN = errors.New("TIN is invalid")

type lookupOpts struct {
	*rootOpts
	lookupType string
}

// tinLookuper is the slice of tin.Client this command uses. It exists so that
// tests can substitute a fake client instead of dialing the live registry.
type tinLookuper interface {
	LookupInvoice(ctx context.Context, inv *bill.Invoice, party tin.InvoiceParty) (*tin.InvoiceResult, error)
}

// newTinClient builds the lookup client. Tests replace it.
var newTinClient = func() tinLookuper { return tin.New() }

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

	res, err := newTinClient().LookupInvoice(cmd.Context(), inv, tin.InvoiceParty(c.lookupType))
	if err != nil {
		return fmt.Errorf("looking up TIN: %w", err)
	}

	allValid := true
	if res.Customer != nil {
		printResult(cmd.OutOrStdout(), "Customer", res.Customer)
		allValid = allValid && res.Customer.Valid
	}
	if res.Supplier != nil {
		printResult(cmd.OutOrStdout(), "Supplier", res.Supplier)
		allValid = allValid && res.Supplier.Valid
	}

	if !allValid {
		return errInvalidTIN
	}
	return nil
}

// printResult writes one status line for a party: validity, source, and the
// registered name when the registry disclosed one.
func printResult(w io.Writer, label string, r *tin.Result) {
	status := "invalid"
	if r.Valid {
		status = "valid"
	}
	line := fmt.Sprintf("%s: TIN is %s (source: %s)", label, status, r.Source)
	if r.Name != "" {
		line += ", name: " + r.Name
	}
	_, _ = fmt.Fprintln(w, line)
}
