// Package main provides a CLI interface for the library
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
)

// build data provided by goreleaser and mage setup
var (
	name    = "gobl.tin"
	version = "dev"
	date    = ""
)

// Exit codes.
const (
	exitValid      = 0
	exitInvalid    = 1
	exitUnverified = 2
	exitUsage      = 3
)

// exitError carries the exit code for a failure whose message is already on
// the terminal, or that needs one printed.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

func main() {
	err := run()
	if err != nil && err.Error() != "" {
		_, _ = fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(exitCode(err))
}

// exitCode maps an error to the process exit code: report outcomes carry
// their own code, everything else is a usage, IO or parse failure.
func exitCode(err error) int {
	if err == nil {
		return exitValid
	}
	var e *exitError
	if errors.As(err, &e) {
		return e.code
	}
	return exitUsage
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := godotenv.Load(".env"); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to load .env file: %w", err)
		}
	}

	return root().cmd().ExecuteContext(ctx)
}
