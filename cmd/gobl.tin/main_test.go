package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"gitlab.com/flimzy/testy"

	"github.com/invopop/gobl"
)

func Test_root(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		err      error
		expected string
		stdin    io.Reader
	}{
		{
			name:     "default customer lookup",
			args:     []string{"lookup", "./test/data/invoice-valid.json"},
			expected: "customer: valid\n",
		},
		{
			name:     "supplier lookup",
			args:     []string{"lookup", "./test/data/invoice-valid.json", "--type", "supplier"},
			expected: "supplier: Tax ID Invalid, tax ID not found in database\n",
		},
		{
			name:     "both lookup",
			args:     []string{"lookup", "./test/data/invoice-valid.json", "--type", "both"},
			expected: "customer: valid\nsupplier: Tax ID Invalid, tax ID not found in database\n",
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
