// Command plumbline is a repo-level AI coding readiness assessment tool.
//
// See SPEC.md at the repo root for the full design. The CLI is the
// primary surface; the Bubble Tea TUI is a peer interface that lands
// in milestone M2.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// Exit codes are part of the public CLI contract — see SPEC.md §4.
const (
	exitOK             = 0
	exitGateFailed     = 1
	exitCannotRun      = 2
	exitConfigError    = 3
	exitInternalErrror = 4
)

// exitError lets a subcommand return an error annotated with the exit
// code that should be reported to the shell. Anything that is not an
// exitError gets mapped to exitInternalErrror.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func errCannotRun(err error) error     { return &exitError{exitCannotRun, err} }
func errConfigError(err error) error   { return &exitError{exitConfigError, err} }    //nolint:unused // wired in M1
func errInternalError(err error) error { return &exitError{exitInternalErrror, err} } //nolint:unused

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the CLI with the given args and returns an exit code.
// Split out from main() so tests can drive it with synthetic args.
func run(args []string, stdout, stderr io.Writer) int {
	root := newRootCmd(stdout, stderr)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)

	err := root.Execute()
	if err == nil {
		return exitOK
	}
	var ee *exitError
	if errors.As(err, &ee) {
		fmt.Fprintf(stderr, "error: %s\n", ee.err.Error())
		return ee.code
	}
	// Unknown command, bad flag, or unannotated subcommand error —
	// SilenceErrors keeps cobra quiet, so we print it ourselves.
	fmt.Fprintf(stderr, "error: %s\n", err.Error())
	return exitConfigError
}

func newRootCmd(stdout, stderr io.Writer) *cobra.Command {
	// The root command is also the default action: `plumbline [path]`
	// (no subcommand) runs the assess pipeline. It binds the same flag
	// set as the assess subcommand so 'plumbline --json /repo' and
	// 'plumbline assess --json /repo' are interchangeable.
	rootFlags := &assessFlags{}

	root := &cobra.Command{
		Use:   "plumbline [path]",
		Short: "Repo-level AI coding readiness assessment based on the ACMM",
		Long: `plumbline assesses a repository's AI Codebase Maturity Model (ACMM) level
by detecting feedback-loop artifacts on disk. It runs deterministic checks —
no LLM calls, no network — and reports which loops exist, which are missing,
and what to add to reach the next level.

Two interfaces at full feature parity:
  • Interactive TUI (Bubble Tea) — default when stdout is a TTY
  • Pure CLI — flag-driven, for LLM tool callers and CI gates

Bare invocation runs the unified scan + score + report pipeline:
  plumbline                # scan ".", TUI on a terminal / brief text otherwise
  plumbline /path/to/repo  # scan a specific repo
  plumbline --json         # machine-readable verdict
  plumbline --fail-below 3 # CI gate

This is the same pipeline as 'plumbline assess'. Use the subcommands
below for narrower operations (inspect a single signal, list signals,
publish schemas, etc.). Run 'plumbline help' for topical guides.`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true, // don't dump usage on every error
		SilenceErrors: true, // we print errors ourselves with the right stream
		RunE:          makeAssessRunE(rootFlags, stdout, stderr),
	}
	bindAssessFlags(root, rootFlags)

	// Replace cobra's built-in `help` command with our topic-help command.
	root.SetHelpCommand(newHelpCmd(stdout))

	root.AddCommand(
		newAssessCmd(stdout, stderr),
		newInspectCmd(stdout, stderr),
		newSignalsCmd(stdout, stderr),
		newExplainCmd(stdout, stderr),
		newSchemaCmd(stdout, stderr),
		newVersionCmd(stdout),
	)

	return root
}
