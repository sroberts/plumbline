package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// signalExplanation is the JSON shape of `explain --json`.
type signalExplanation struct {
	ID     string     `json:"id"`
	Level  acmm.Level `json:"level"`
	Family string     `json:"family"`
	Title  string     `json:"title"`
}

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
  plumbline explain l2.agent-instructions
  plumbline explain l3.coverage-gate --json

See also:
  plumbline inspect <id>     run a scan and report this signal's status
  plumbline signals          list every known signal`,
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			ids := make([]string, 0)
			for _, s := range signals.Default.All() {
				ids = append(ids, s.ID())
			}
			return ids, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			s, ok := signals.Default.Get(id)
			if !ok {
				return errCannotRun(fmt.Errorf("unknown signal %q (run 'plumbline signals' for the full list)", id))
			}
			ex := signalExplanation{
				ID:     s.ID(),
				Level:  s.Level(),
				Family: s.Family(),
				Title:  s.Title(),
			}
			if asJSON {
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(ex)
			}
			fmt.Fprintf(stdout, "%s — %s\n", ex.ID, ex.Title)
			fmt.Fprintf(stdout, "Level:   %d (%s)\n", ex.Level, ex.Level.Name())
			fmt.Fprintf(stdout, "Family:  %s\n", ex.Family)
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "See also:")
			fmt.Fprintf(stdout, "  plumbline inspect %s [path]   to scan and report status\n", ex.ID)
			fmt.Fprintln(stdout, "  plumbline help signals         lifecycle and partial-credit semantics")
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Emit the description as JSON.")
	return cmd
}
