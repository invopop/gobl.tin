package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/invopop/gobl"
	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl.tin/vies"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/org"
	"github.com/spf13/cobra"
)

// errNotValid makes the command exit non-zero when a printed report is not
// valid. The report lines are already on stdout by then.
var errNotValid = errors.New("verification failed")

// Party selectors for an invoice.
const (
	partyCustomer = "customer"
	partySupplier = "supplier"
	partyBoth     = "both"
)

type verifyOpts struct {
	*rootOpts
	party string
	json  bool

	// verifiers is the ordered set the command verifies with. Nil means the
	// production set, VIES; tests inject fakes so that no command dials the
	// register.
	verifiers []tin.Verifier
}

// labelled pairs a report with the name of the party it describes.
type labelled struct {
	label  string
	report *tin.Report
}

func verify(o *rootOpts) *verifyOpts {
	return &verifyOpts{rootOpts: o}
}

func (c *verifyOpts) cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify <file>",
		Short: "Verify the identifiers of a party, or of an invoice's parties, against their registers",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runE,
	}

	cmd.Flags().StringVar(&c.party, "party", partyCustomer, "Invoice party to verify: customer, supplier or both")
	cmd.Flags().BoolVar(&c.json, "json", false, "Print the report as JSON")

	return cmd
}

func (c *verifyOpts) runE(cmd *cobra.Command, args []string) error {
	data, err := readInput(cmd, args[0])
	if err != nil {
		return err
	}

	parties, err := c.selectParties(data)
	if err != nil {
		return err
	}

	verifiers := c.verifiers
	if verifiers == nil {
		verifiers = []tin.Verifier{vies.New()}
	}
	client := tin.New(verifiers...)

	reports := make([]labelled, 0, len(parties))
	for _, p := range parties {
		report, err := client.Verify(cmd.Context(), p.party)
		if err != nil {
			return fmt.Errorf("%s: %w", p.label, err)
		}
		reports = append(reports, labelled{label: p.label, report: report})
	}

	if c.json {
		if err := printJSON(cmd.OutOrStdout(), reports); err != nil {
			return err
		}
	} else {
		printReports(cmd.OutOrStdout(), reports)
	}

	for _, r := range reports {
		if !r.report.Valid() {
			return errNotValid
		}
	}
	return nil
}

// selected is one party to verify with the label it is printed under. The
// label is empty for a party document.
type selected struct {
	label string
	party *org.Party
}

// selectParties parses the input as a GOBL envelope or document and picks the
// parties the --party flag asks for.
func (c *verifyOpts) selectParties(data []byte) ([]selected, error) {
	doc, err := gobl.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parsing input: %w", err)
	}
	if env, ok := doc.(*gobl.Envelope); ok {
		doc = env.Extract()
	}

	switch d := doc.(type) {
	case *org.Party:
		return []selected{{party: d}}, nil
	case *bill.Invoice:
		return c.selectInvoiceParties(d)
	default:
		return nil, fmt.Errorf("unsupported document type %T", doc)
	}
}

// selectInvoiceParties picks the invoice parties the --party flag asks for.
func (c *verifyOpts) selectInvoiceParties(inv *bill.Invoice) ([]selected, error) {
	var out []selected
	if c.party == partyCustomer || c.party == partyBoth {
		if inv.Customer == nil {
			return nil, errors.New("invoice has no customer")
		}
		out = append(out, selected{label: partyCustomer, party: inv.Customer})
	}
	if c.party == partySupplier || c.party == partyBoth {
		if inv.Supplier == nil {
			return nil, errors.New("invoice has no supplier")
		}
		out = append(out, selected{label: partySupplier, party: inv.Supplier})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("invalid party %q, expected customer, supplier or both", c.party)
	}
	if len(out) == 1 {
		out[0].label = ""
	}
	return out, nil
}

// readInput reads the file, or stdin when the name is "-".
func readInput(cmd *cobra.Command, name string) ([]byte, error) {
	if name == "-" {
		return io.ReadAll(cmd.InOrStdin())
	}
	return os.ReadFile(name)
}

// printJSON writes one report, or an object keyed by party label when more
// than one party is verified.
func printJSON(w io.Writer, reports []labelled) error {
	enc := json.NewEncoder(w)
	if len(reports) == 1 {
		return enc.Encode(reports[0].report)
	}
	out := make(map[string]*tin.Report, len(reports))
	for _, r := range reports {
		out[r.label] = r.report
	}
	return enc.Encode(out)
}

// printReports writes one line per check and one indented line per
// mismatch. A labelled report is headed by its label and indented.
func printReports(w io.Writer, reports []labelled) {
	for _, r := range reports {
		indent := ""
		if r.label != "" {
			_, _ = fmt.Fprintf(w, "%s:\n", r.label)
			indent = "  "
		}
		for _, c := range r.report.Checks {
			_, _ = fmt.Fprintf(w, "%s%s\n", indent, checkLine(c))
			for _, m := range c.Mismatches {
				_, _ = fmt.Fprintf(w, "%s  %s: document %q, register %q\n", indent, m.Path, m.Document, m.Register)
			}
		}
	}
}

// checkLine formats one check as "<path>: <status> (<source>)". An
// unsupported check has no source; an unverified one ends with the failure.
func checkLine(c *tin.Check) string {
	line := fmt.Sprintf("%s: %s", c.Path, c.Status)
	if c.Source != "" {
		line += fmt.Sprintf(" (%s)", c.Source)
	}
	if c.Status == tin.StatusUnverified && c.Failure != "" {
		line += ": " + c.Failure
	}
	return line
}
