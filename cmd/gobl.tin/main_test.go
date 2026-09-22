package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"gitlab.com/flimzy/testy"

	"github.com/invopop/gobl"
	tin "github.com/invopop/gobl.tin"
	"github.com/invopop/gobl/bill"
)

// fakeClient implements tinLookuper with a canned result per party.
type fakeClient struct {
	result func(party tin.InvoiceParty) (*tin.InvoiceResult, error)
}

func (f *fakeClient) LookupInvoice(_ context.Context, _ *bill.Invoice, party tin.InvoiceParty) (*tin.InvoiceResult, error) {
	return f.result(party)
}

// withFakeClient swaps the CLI's client for the duration of a test.
func withFakeClient(t *testing.T, result func(party tin.InvoiceParty) (*tin.InvoiceResult, error)) {
	t.Helper()
	orig := newTinClient
	newTinClient = func() tinLookuper { return &fakeClient{result: result} }
	t.Cleanup(func() { newTinClient = orig })
}

// resultsFor builds an InvoiceResult with the given per-party results,
// honouring the requested party selector like the real client does.
func resultsFor(customer, supplier *tin.Result) func(tin.InvoiceParty) (*tin.InvoiceResult, error) {
	return func(party tin.InvoiceParty) (*tin.InvoiceResult, error) {
		out := new(tin.InvoiceResult)
		if party == tin.InvoicePartyCustomer || party == tin.InvoicePartyBoth {
			out.Customer = customer
		}
		if party == tin.InvoicePartySupplier || party == tin.InvoicePartyBoth {
			out.Supplier = supplier
		}
		if out.Customer == nil && out.Supplier == nil {
			return nil, tin.ErrInput.WithMsgf("invalid party %q", party)
		}
		return out, nil
	}
}

func Test_root(t *testing.T) {
	valid := &tin.Result{Valid: true, Source: "vies"}
	validNamed := &tin.Result{Valid: true, Name: "ACME GMBH", Source: "vies"}
	invalid := &tin.Result{Valid: false, Source: "vies"}

	tests := []struct {
		name     string
		args     []string
		result   func(tin.InvoiceParty) (*tin.InvoiceResult, error)
		err      error
		expected string
	}{
		{
			name:     "default customer lookup",
			args:     []string{"lookup", "../../test/data/invoice-valid.json"},
			result:   resultsFor(valid, valid),
			expected: "Customer: TIN is valid (source: vies)\n",
		},
		{
			name:     "customer lookup with name",
			args:     []string{"lookup", "../../test/data/invoice-valid.json"},
			result:   resultsFor(validNamed, valid),
			expected: "Customer: TIN is valid (source: vies), name: ACME GMBH\n",
		},
		{
			name:     "supplier lookup",
			args:     []string{"lookup", "../../test/data/invoice-valid.json", "--type", "supplier"},
			result:   resultsFor(valid, valid),
			expected: "Supplier: TIN is valid (source: vies)\n",
		},
		{
			name:     "both lookup",
			args:     []string{"lookup", "../../test/data/invoice-valid.json", "--type", "both"},
			result:   resultsFor(valid, valid),
			expected: "Customer: TIN is valid (source: vies)\nSupplier: TIN is valid (source: vies)\n",
		},
		{
			name:     "invalid TIN exits non-zero",
			args:     []string{"lookup", "../../test/data/invoice-valid.json", "--type", "supplier"},
			result:   resultsFor(valid, invalid),
			err:      errInvalidTIN,
			expected: "Supplier: TIN is invalid (source: vies)\n",
		},
		{
			name:   "lookup error",
			args:   []string{"lookup", "../../test/data/invoice-valid.json"},
			result: resultsFor(nil, nil),
			err:    fmt.Errorf(`looking up TIN: input: invalid party "customer"`),
		},
		{
			name: "lookup no args",
			args: []string{"lookup"},
			err:  fmt.Errorf("expected exactly one input file, the command usage is `gobl.tin lookup <input>`"),
		},
		{
			name: "lookup too many args",
			args: []string{"lookup", "foo", "bar"},
			err:  fmt.Errorf("expected exactly one input file, the command usage is `gobl.tin lookup <input>`"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result != nil {
				withFakeClient(t, tt.result)
			}

			cmd := &cobra.Command{SilenceUsage: true, SilenceErrors: true}
			rootOpts := &rootOpts{}
			lookupCmd := lookup(rootOpts).cmd()

			cmd.AddCommand(lookupCmd)
			output := &bytes.Buffer{}
			cmd.SetOut(output)
			cmd.SetErr(output)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if tt.err != nil {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.err.Error())
			} else {
				assert.NoError(t, err)
			}
			if tt.expected != "" {
				assert.Equal(t, tt.expected, output.String())
			}
		})
	}
}

func Test_version(t *testing.T) {
	cmd := versionCmd()
	stdout, stderr := testy.RedirIO(nil, func() {
		err := cmd.Execute()
		if err != nil {
			t.Fatal(err)
		}
	})
	wantOut := string(gobl.VERSION) // just check it's there somewhere!
	wantErr := ""
	if sout, _ := io.ReadAll(stdout); !strings.Contains(string(sout), wantOut) {
		t.Errorf("Unexpected STDOUT: %s", sout)
	}
	if serr, _ := io.ReadAll(stderr); !strings.Contains(string(serr), wantErr) {
		t.Errorf("Unexpected STDERR: %s", serr)
	}
}
