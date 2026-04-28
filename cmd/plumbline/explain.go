package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newExplainCmd(stdout, stderr io.Writer) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "explain <signal-id>",
		Short: "Print a signal's description, detection rule, and rationale (no scan)",
		Long: `plumbline explain — static description of a signal.

Prints the signal's title, description, detection rule summary, and the
rationale for why it matters at its level. Does not scan a repository;
use 'plumbline inspect' for that.

Examples:
  plumbline explain l2.claude-md
  plumbline explain l3.coverage-gate --json

See also:
  plumbline inspect <id>     run a scan and report this signal's status
  plumbline signals          list every known signal`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = args
			_ = asJSON
			fmt.Fprintln(stderr, "(M1: explain is wired up; signals populate with the next PR.)")
			return errCannotRun(errors.New("not implemented in this milestone"))
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Emit the description as JSON.")
	return cmd
}
