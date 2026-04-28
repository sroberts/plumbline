package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
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
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveDefault
			}
			ids := make([]string, 0)
			for _, s := range signals.Default.All() {
				ids = append(ids, s.ID())
			}
			return ids, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = debug
			id := args[0]
			path := "."
			if len(args) > 1 {
				path = args[1]
			}

			sig, ok := signals.Default.Get(id)
			if !ok {
				return errCannotRun(fmt.Errorf("unknown signal %q (run 'plumbline signals' for the full list)", id))
			}

			abs, err := filepath.Abs(path)
			if err != nil {
				return errCannotRun(err)
			}
			idx, err := scanner.Scan(abs)
			if err != nil {
				return errCannotRun(err)
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			result := sig.Detect(ctx, idx)

			detail := acmm.SignalResult{
				ID:         sig.ID(),
				Level:      sig.Level(),
				Family:     sig.Family(),
				Title:      sig.Title(),
				Status:     result.Status,
				Score:      result.Score,
				Confidence: result.Confidence,
				Method:     result.Method,
				Evidence:   result.Evidence,
				Notes:      result.Notes,
				Diag:       result.Diag,
			}

			if asJSON {
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(detail)
			}
			writeInspectText(stdout, detail, abs)
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&asJSON, "json", false, "Emit a single signal-result JSON object.")
	f.BoolVar(&debug, "debug", false, "Include detection diagnostics in output.")
	return cmd
}

func writeInspectText(w io.Writer, r acmm.SignalResult, repoPath string) {
	fmt.Fprintf(w, "%s · %s\n", r.ID, r.Title)
	fmt.Fprintf(w, "Repo:        %s\n", repoPath)
	fmt.Fprintf(w, "Level:       %d (%s)\n", r.Level, r.Level.Name())
	fmt.Fprintf(w, "Family:      %s\n", r.Family)
	fmt.Fprintf(w, "Status:      %s   (score=%v  confidence=%s  method=%s)\n",
		r.Status, r.Score, r.Confidence, r.Method)
	fmt.Fprintln(w)

	if len(r.Evidence) == 0 {
		fmt.Fprintln(w, "Evidence: (none)")
	} else {
		fmt.Fprintln(w, "Evidence:")
		for _, e := range r.Evidence {
			if e.Excerpt != "" {
				fmt.Fprintf(w, "  %s\n    %q\n", e.Path, e.Excerpt)
			} else {
				fmt.Fprintf(w, "  %s\n", e.Path)
			}
		}
	}

	if len(r.Notes) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Notes:")
		for _, n := range r.Notes {
			fmt.Fprintf(w, "  %s\n", n)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "See also:")
	fmt.Fprintf(w, "  plumbline explain %s\n", r.ID)
	fmt.Fprintln(w, "  plumbline help scoring")
}
