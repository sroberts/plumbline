package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newSignalsCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		asJSON       bool
		levelFilters []int
		familyFilter []string
	)

	cmd := &cobra.Command{
		Use:   "signals",
		Short: "List every signal the tool knows how to detect",
		Long: `plumbline signals — list every signal the tool knows how to detect.

Static — does not scan a repo. Filterable by ACMM level and family. The
JSON output is the canonical way for an LLM agent to discover what's
detectable before calling 'assess'.

Examples:
  # All signals, grouped by level.
  plumbline signals

  # Only L3 signals in the coverage family.
  plumbline signals --level 3 --family coverage

  # Machine-readable for tool callers.
  plumbline signals --json

See also:
  plumbline explain <id>     long-form description of one signal
  plumbline help signals     signal lifecycle and partial-credit semantics`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = args
			_ = asJSON
			_ = levelFilters
			_ = familyFilter
			fmt.Fprintln(stderr, "(M1: signals is wired up; the registry populates with the next PR.)")
			return errCannotRun(errors.New("not implemented in this milestone"))
		},
	}

	f := cmd.Flags()
	f.BoolVar(&asJSON, "json", false, "Emit the registry as JSON to stdout.")
	f.IntSliceVar(&levelFilters, "level", nil, "Only list signals at level N (2-5). Repeatable.")
	f.StringSliceVar(&familyFilter, "family", nil, "Only list signals in family <name>. Repeatable.")
	return cmd
}
