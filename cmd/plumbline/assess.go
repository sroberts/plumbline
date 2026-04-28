package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/sroberts/plumbline/internal/buildinfo"
	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/scoring"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

func newAssessCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		asJSON        bool
		reportFmt     string
		outPath       string
		eventsFmt     string
		quiet         bool
		noColor       bool
		cli           bool
		tui           bool
		failBelow     int
		profile       string
		configPath    string
		minConfidence string
		signalSet     string
		ciSystem      string
		debug         bool
		clock         string
		includeSignal []string
		excludeSignal []string
		levelFilters  []int
		familyFilters []string
	)

	cmd := &cobra.Command{
		Use:   "assess [path]",
		Short: "Scan a repo and report its ACMM maturity level",
		Long: `plumbline assess — scan a repo and report its ACMM maturity level.

Walks the repository at [path] (default ".") and runs every enabled
signal detector against it. Produces a verdict (level 1-5), per-level
scores, and the list of signals that would unlock the next level.

Mode is auto-detected: TUI on a terminal, CLI when piped or in CI.
Use --cli to force non-interactive output, --tui to force the TUI.

Examples:
  # Interactive TUI: scan the current directory.
  plumbline assess

  # Machine-readable, with progress events on stderr.
  plumbline assess --json --events ndjson 2>events.log >verdict.json

  # CI gate: fail if not at level 3 or higher.
  plumbline assess --fail-below 3 --quiet

  # Scan a specific repo, save markdown report.
  plumbline assess /path/to/repo --report markdown --out maturity.md

  # Only run the L3 coverage signals.
  plumbline assess --level 3 --family coverage --json

Exit codes:
  0  scan completed; gate passed (or no gate set)
  1  scan completed; gate failed (assessed level < --fail-below)
  2  could not run (path not a directory, IO error)
  3  configuration error

See also:
  plumbline inspect          drill into one signal's evidence
  plumbline signals          list every signal the tool detects
  plumbline help scoring     how levels and the no-skip rule are computed
  plumbline help agents      guidance for LLM tool callers`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cli && tui {
				return errCannotRun(errors.New("--cli and --tui are mutually exclusive"))
			}
			if len(includeSignal) > 0 && len(excludeSignal) > 0 {
				return errCannotRun(errors.New("--include-signal and --exclude-signal are mutually exclusive"))
			}

			path := "."
			if len(args) == 1 {
				path = args[0]
			}

			confLevel, err := parseConfidence(minConfidence)
			if err != nil {
				return errCannotRun(err)
			}

			report, err := runAssess(cmd.Context(), path, scoring.Options{MinConfidence: confLevel})
			if err != nil {
				return errCannotRun(err)
			}

			if asJSON {
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return errCannotRun(err)
				}
				return nil
			}

			// Default human-readable text. Fuller layout (level bars,
			// next-gap panel) lands in the next PR; for now keep it
			// short so something useful prints in non-JSON mode.
			writeBriefText(stdout, report)
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&asJSON, "json", false, "Emit JSON to stdout. Implies --cli. Schema: 'plumbline schema verdict'.")
	f.StringVar(&reportFmt, "report", "", "Write a report file. fmt: markdown|json|sarif. Implies --cli.")
	f.StringVar(&outPath, "out", "-", "Report destination. \"-\" = stdout.")
	f.StringVar(&eventsFmt, "events", "", "Stream progress events to stderr. fmt: ndjson|text. Schema: 'plumbline schema event'.")
	f.BoolVarP(&quiet, "quiet", "q", false, "Suppress banners, progress, and trailing hints. Implies --cli.")
	f.BoolVar(&noColor, "no-color", false, "Disable ANSI color (also via NO_COLOR=1).")
	f.BoolVar(&cli, "cli", false, "Force pure-CLI mode. Auto-set when stdout is not a TTY.")
	f.BoolVar(&tui, "tui", false, "Force TUI even when stdout is not a TTY.")
	f.IntVar(&failBelow, "fail-below", 0, "Exit 1 if assessed level < N (2-5). 0 = no gate.")
	f.StringVar(&profile, "profile", "default", "Named signal preset. See 'plumbline help profiles'.")
	f.StringVar(&configPath, "config", "", "Override config path. Default: .plumbline.yml.")
	f.StringVar(&minConfidence, "min-confidence", "low", "Minimum confidence to credit a signal: low|medium|high.")
	f.StringVar(&signalSet, "signal-set", "latest", "Pin signal rule-set version (e.g., v1).")
	f.StringVar(&ciSystem, "ci-system", "auto", "CI flavor: auto|github-actions. Other systems are M4+.")
	f.BoolVar(&debug, "debug", false, "Emit detection diagnostics on stderr. Implies --cli.")
	f.StringVar(&clock, "clock", "wall", "Timestamp source for events: wall|fixed|relative.")
	f.StringSliceVar(&includeSignal, "include-signal", nil, "Only run the listed signals. Repeatable.")
	f.StringSliceVar(&excludeSignal, "exclude-signal", nil, "Skip the listed signals. Repeatable.")
	f.IntSliceVar(&levelFilters, "level", nil, "Only run signals at level N (2-5). Repeatable.")
	f.StringSliceVar(&familyFilters, "family", nil, "Only run signals in family <name>. Repeatable.")

	return cmd
}

// runAssess executes the scan -> detect -> score pipeline and returns
// a populated Report. Pulled out of RunE so it stays testable in
// isolation when more flags arrive (filters, profiles, --enrich, etc.).
func runAssess(ctx context.Context, path string, opts scoring.Options) (acmm.Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return acmm.Report{}, err
	}

	idx, err := scanner.Scan(abs)
	if err != nil {
		return acmm.Report{}, err
	}

	registered := signals.Default.All()
	results := make([]acmm.SignalResult, 0, len(registered))
	for _, s := range registered {
		r := s.Detect(ctx, idx)
		results = append(results, acmm.SignalResult{
			ID:         s.ID(),
			Level:      s.Level(),
			Family:     s.Family(),
			Title:      s.Title(),
			Status:     r.Status,
			Score:      r.Score,
			Confidence: r.Confidence,
			Method:     r.Method,
			Evidence:   r.Evidence,
			Notes:      r.Notes,
			Diag:       r.Diag,
		})
	}

	verdict := scoring.Compute(results, opts)

	return acmm.Report{
		Schema:           buildinfo.Schema,
		ToolVersion:      buildinfo.Version,
		SignalSetVersion: buildinfo.SignalSetVersion,
		CISystem:         "github-actions",
		Repo:             abs,
		ScannedAt:        time.Now().UTC().Format(time.RFC3339),
		Verdict:          verdict,
		Signals:          results,
	}, nil
}

func parseConfidence(s string) (acmm.Confidence, error) {
	switch s {
	case "", "low":
		return acmm.ConfidenceLow, nil
	case "medium":
		return acmm.ConfidenceMedium, nil
	case "high":
		return acmm.ConfidenceHigh, nil
	default:
		return "", fmt.Errorf("invalid --min-confidence %q (want low|medium|high)", s)
	}
}

// writeBriefText prints a one-screen summary of the verdict. The richer
// layout (level bars, next-gap panel) lands in a follow-up PR.
func writeBriefText(w io.Writer, r acmm.Report) {
	fmt.Fprintf(w, "plumbline · %s\n", r.Repo)
	fmt.Fprintf(w, "Assessed level: %d (%s)\n\n", r.Verdict.Level, r.Verdict.Name)
	for _, l := range []acmm.Level{
		acmm.LevelInstructed,
		acmm.LevelMeasured,
		acmm.LevelAdaptive,
		acmm.LevelSelfSustaining,
	} {
		fmt.Fprintf(w, "  L%d %-16s  %5.1f%%\n", l, l.Name(), r.Verdict.LevelScores[l]*100)
	}
	if len(r.Verdict.NextGap) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Next-level gap (to reach L%d):\n", r.Verdict.Level+1)
		for _, id := range r.Verdict.NextGap {
			fmt.Fprintf(w, "  · %s\n", id)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Hint: re-run with --json for machine-readable output.")
}
