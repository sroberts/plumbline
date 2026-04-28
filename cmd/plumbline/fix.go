package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sroberts/plumbline/internal/fix"
	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/internal/textwrap"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// fixerForID returns the Fixer-implementing signal with the given ID,
// or false if either the ID is unknown or the signal cannot fix itself.
func fixerForID(id string) (signals.Fixer, bool) {
	sig, ok := signals.Default.Get(id)
	if !ok {
		return nil, false
	}
	fxr, ok := sig.(signals.Fixer)
	return fxr, ok
}

func newFixCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		apply   bool
		asJSON  bool
		repoArg string
		inputs  []string
	)

	cmd := &cobra.Command{
		Use:   "fix <signal-id> [path]",
		Short: "Plan or apply a scaffolded fix for a signal",
		Long: `plumbline fix — generate or apply a scaffolded fix for a signal.

By default this is a dry-run: prints the FixPlan (file paths and
contents) without touching the filesystem. Pass --apply to actually
write the files. Refuses to overwrite existing files; will append to
existing instruction files when appropriate.

Inputs the fix needs (e.g. project conventions) can be supplied with
repeatable --input KEY=VALUE pairs. Missing inputs use the fix's
default placeholders (which include "TODO:" markers so you remember
to fill them in).

Examples:
  # See what 'fix' would write for the missing CLAUDE.md.
  plumbline fix l2.claude-md

  # Actually scaffold CLAUDE.md, providing two inputs up front.
  plumbline fix l2.claude-md --apply \
      --input "project_summary=A Go CLI for X." \
      --input "conventions=- Use UV for Python envs.\n- No raw SQL."

  # JSON for an LLM tool harness.
  plumbline fix l2.claude-md --json

  # Fix in a different repo.
  plumbline fix l2.pr-template /path/to/repo --apply

Exit codes:
  0  plan generated (or applied successfully)
  2  could not run (signal id unknown, signal can't fix, path bad)
  3  configuration error (bad --input, etc.)

Safety:
  • paths in the FixPlan must stay inside the repo root
  • create-file refuses to overwrite an existing file
  • append-file requires the target to already exist
  • dry-run is the default; --apply is required to write
`,
		Args: cobra.RangeArgs(1, 2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveDefault
			}
			ids := make([]string, 0)
			for _, s := range signals.Default.All() {
				if _, ok := s.(signals.Fixer); ok {
					ids = append(ids, s.ID())
				}
			}
			return ids, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			repoArg = "."
			if len(args) > 1 {
				repoArg = args[1]
			}

			fxr, ok := fixerForID(id)
			if !ok {
				if _, exists := signals.Default.Get(id); exists {
					return errCannotRun(fmt.Errorf("signal %q has no fixer (not all signals can scaffold themselves)", id))
				}
				return errCannotRun(fmt.Errorf("unknown signal %q (run 'plumbline signals' for the list)", id))
			}

			abs, err := filepath.Abs(repoArg)
			if err != nil {
				return errCannotRun(err)
			}
			idx, err := scanner.Scan(abs)
			if err != nil {
				return errCannotRun(err)
			}

			parsed, err := parseInputs(inputs)
			if err != nil {
				return errCannotRun(err)
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			plan, err := fxr.Plan(ctx, idx, parsed)
			if err != nil {
				return errCannotRun(fmt.Errorf("plan: %w", err))
			}

			result, err := fix.Apply(abs, plan, fix.Options{DryRun: !apply})
			if err != nil {
				return errCannotRun(err)
			}

			if asJSON {
				return emitFixJSON(stdout, plan, result, !apply)
			}
			emitFixText(stdout, plan, result, abs, !apply)
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&apply, "apply", false, "Actually write the files (default is dry-run).")
	f.BoolVar(&asJSON, "json", false, "Emit plan + result as JSON.")
	f.StringSliceVar(&inputs, "input", nil, "User-supplied input as KEY=VALUE. Repeatable.")
	return cmd
}

// parseInputs converts --input KEY=VALUE pairs into a map.
func parseInputs(raw []string) (map[string]string, error) {
	out := make(map[string]string, len(raw))
	for _, kv := range raw {
		i := strings.IndexByte(kv, '=')
		if i < 0 {
			return nil, fmt.Errorf("--input %q: expected KEY=VALUE", kv)
		}
		key := kv[:i]
		val := kv[i+1:]
		if key == "" {
			return nil, fmt.Errorf("--input %q: empty key", kv)
		}
		// Allow \n in CLI input to mean a real newline.
		val = strings.ReplaceAll(val, "\\n", "\n")
		out[key] = val
	}
	return out, nil
}

// emitFixText prints a human-readable plan + result.
func emitFixText(w io.Writer, plan acmm.FixPlan, res fix.Result, repoRoot string, dryRun bool) {
	fmt.Fprintf(w, "plumbline fix · %s\n", plan.SignalID)
	fmt.Fprintf(w, "Repo:   %s\n", repoRoot)
	mode := "DRY-RUN (use --apply to write)"
	if !dryRun {
		mode = "APPLIED"
	}
	fmt.Fprintf(w, "Mode:   %s\n\n", mode)

	if plan.Summary != "" {
		fmt.Fprintln(w, textwrap.Wrap(plan.Summary, 78))
		fmt.Fprintln(w)
	}

	for i, op := range plan.Ops {
		opRes := fix.OpResult{}
		if i < len(res.Operations) {
			opRes = res.Operations[i]
		}
		fmt.Fprintf(w, "[%s] %s\n", op.Kind, op.Path)
		switch {
		case opRes.Wrote:
			fmt.Fprintf(w, "  → wrote %d bytes\n", opRes.Bytes)
		case dryRun:
			fmt.Fprintf(w, "  → would write %d bytes\n", opRes.Bytes)
		}
		if op.Kind == acmm.FixCreateFile || op.Kind == acmm.FixAppendFile {
			fmt.Fprintln(w, "  ─── content ───")
			for _, line := range strings.Split(string(op.Body), "\n") {
				fmt.Fprintf(w, "  | %s\n", line)
			}
			fmt.Fprintln(w, "  ─── end ───")
		}
	}

	if dryRun {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Hint: re-run with --apply to write the file(s).")
	}
}

func emitFixJSON(w io.Writer, plan acmm.FixPlan, res fix.Result, dryRun bool) error {
	out := struct {
		Plan   acmm.FixPlan `json:"plan"`
		Result fix.Result   `json:"result"`
		DryRun bool         `json:"dry_run"`
	}{Plan: plan, Result: res, DryRun: dryRun}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// Sanity check the package import side; keep this here so 'unused
// import' doesn't fire if Apply is called only inside the closure.
var _ = os.WriteFile
