package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newInspectCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		asJSON bool
		debug  bool
	)

	cmd := &cobra.Command{
		Use:   "inspect <signal-id> [path]",
		Short: "Run a scan and print the detail view for one signal",
		Long: `plumbline inspect — run a scan and print the detail view for a single signal.

CLI equivalent of opening the TUI's detail screen for that signal: shows
the signal's status, score, confidence, evidence, and a fix recipe.

Examples:
  # Inspect why CLAUDE.md detection said what it said.
  plumbline inspect l2.claude-md

  # JSON for an LLM agent to parse.
  plumbline inspect l3.coverage-gate --json

  # Inspect a signal in a different repo.
  plumbline inspect l2.pr-template /path/to/repo

Exit codes:
  0  signal detail printed
  2  could not run (signal id unknown, path not a repo)
  3  configuration error

See also:
  plumbline explain <id>     static description without scanning
  plumbline signals          list every known signal id`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = args
			_ = debug
			fmt.Fprintln(stderr, "(M1: inspect is wired up; signal execution lands with the scanner.)")
			fmt.Fprintln(stderr, "Hint: 'plumbline signals' will list ids once the registry is populated.")
			return errCannotRun(errors.New("not implemented in this milestone"))
		},
	}

	f := cmd.Flags()
	f.BoolVar(&asJSON, "json", false, "Emit a single signal-result JSON object.")
	f.BoolVar(&debug, "debug", false, "Include detection diagnostics in output.")
	return cmd
}
