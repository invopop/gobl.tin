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
)

// fakeClient implements tinLookuper with a canned answer per input.
type fakeClient struct {
	lookup func(in any) error
}

func (f *fakeClient) Lookup(_ context.Context, in any) error {
	return f.lookup(in)
}

// withFakeClient swaps the CLI's client for the duration of a test.
func withFakeClient(t *testing.T, lookup func(in any) error) {
	t.Helper()
	orig := newTinClient
	newTinClient = func() tinLookuper { return &fakeClient{lookup: lookup} }
	t.Cleanup(func() { newTinClient = orig })
}

func Test_root(t *testing.T) {
	allValid := func(any) error { return nil }
	invalid := func(any) error { return tin.ErrInvalid.WithMessage("TIN is invalid") }

	tests := []struct {
		name     string
		args     []string
		lookup   func(in any) error
		err      error
		expected string
	}{
		{
			name:     "default customer lookup",
			args:     []string{"lookup", "../../test/data/invoice-valid.json"},
			lookup:   allValid,
			expected: "Customer: TIN is valid\n",
		},
		{
			name:     "supplier lookup",
			args:     []string{"lookup", "../../test/data/invoice-valid.json", "--type", "supplier"},
			lookup:   allValid,
			expected: "Supplier: TIN is valid",
		},
		{
			name:     "both lookup",
			args:     []string{"lookup", "../../test/data/invoice-valid.json", "--type", "both"},
			lookup:   allValid,
			expected: "Customer: TIN is valid\nSupplier: Tax ID is valid",
		},
		{
			name:     "invalid TIN",
			args:     []string{"lookup", "../../test/data/invoice-valid.json", "--type", "supplier"},
			lookup:   invalid,
			expected: "TIN is invalid",
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if tt.lookup != nil {
				withFakeClient(t, tt.lookup)
			}

			cmd := &cobra.Command{}
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
