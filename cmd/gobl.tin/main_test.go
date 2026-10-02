package main

import (
	"io"
	"strings"
	"testing"

	"gitlab.com/flimzy/testy"

	"github.com/invopop/gobl"
)

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
