package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"golang.org/x/term"

	"github.com/sroberts/plumbline/internal/buildinfo"
	"github.com/sroberts/plumbline/internal/report"
	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/scoring"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/internal/tui"
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
		forceTUI      bool
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
			if cli && forceTUI {
				return errCannotRun(errors.New("--cli and --tui are mutually exclusive"))
			}
			if len(includeSignal) > 0 && len(excludeSignal) > 0 {
				return errCannotRun(errors.New("--include-signal and --exclude-signal are mutually exclusive"))
			}
			if reportFmt != "" && reportFmt != "json" && reportFmt != "markdown" && reportFmt != "sarif" {
				return errCannotRun(fmt.Errorf("invalid --report %q (want json|markdown|sarif)", reportFmt))
			}
			if eventsFmt != "" && eventsFmt != "ndjson" && eventsFmt != "text" {
				return errCannotRun(fmt.Errorf("invalid --events %q (want ndjson|text)", eventsFmt))
			}

			path := "."
			if len(args) == 1 {
				path = args[0]
			}

			confLevel, err := parseConfidence(minConfidence)
			if err != nil {
				return errCannotRun(err)
			}

			emitter := report.NewEventEmitter(stderr, eventsFmt == "ndjson")

			pipelineOpts := pipelineOptions{
				IncludeSignal: includeSignal,
				ExcludeSignal: excludeSignal,
				LevelFilters:  levelFilters,
				FamilyFilters: familyFilters,
				Scoring:       scoring.Options{MinConfidence: confLevel},
				Events:        emitter,
				Debug:         debug,
				DebugStderr:   stderr,
			}

			// Mode selection per SPEC.md §4.
			modeIsTUI := forceTUI || (!cli && !asJSON && reportFmt == "" && eventsFmt == "" && !quiet && !debug && stdoutIsTerminal(stdout))

			if modeIsTUI {
				rpt, err := tui.Run(func(ctx context.Context) (acmm.Report, *scanner.RepoIndex, error) {
					return runAssessWithIndex(ctx, path, pipelineOpts)
				})
				if err != nil {
					return errCannotRun(err)
				}
				if failBelow > 0 && int(rpt.Verdict.Level) < failBelow {
					return &exitError{code: exitGateFailed, err: fmt.Errorf("verdict level %d is below --fail-below %d", rpt.Verdict.Level, failBelow)}
				}
				return nil
			}

			rpt, err := runAssess(cmd.Context(), path, pipelineOpts)
			if err != nil {
				return errCannotRun(err)
			}

			// --report writes to a file (or "-" = stdout).
			if reportFmt != "" {
				if err := writeReport(rpt, reportFmt, outPath, stdout); err != nil {
					return errCannotRun(err)
				}
			} else if asJSON {
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(rpt); err != nil {
					return errCannotRun(err)
				}
			} else if !quiet {
				writeBriefText(stdout, rpt)
			}

			// --fail-below gating: exit 1 if level is below the floor.
			if failBelow > 0 && int(rpt.Verdict.Level) < failBelow {
				return &exitError{
					code: exitGateFailed,
					err:  fmt.Errorf("verdict level %d is below --fail-below %d", rpt.Verdict.Level, failBelow),
				}
			}
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
	f.BoolVar(&forceTUI, "tui", false, "Force TUI even when stdout is not a TTY.")
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

// pipelineOptions consolidates the flags assess passes to runAssess.
type pipelineOptions struct {
	IncludeSignal []string
	ExcludeSignal []string
	LevelFilters  []int
	FamilyFilters []string
	Scoring       scoring.Options
	Events        *report.EventEmitter
	Debug         bool
	DebugStderr   io.Writer
}

func (p *pipelineOptions) shouldRun(id string, level acmm.Level, family string) bool {
	if len(p.IncludeSignal) > 0 {
		for _, want := range p.IncludeSignal {
			if want == id {
				return true
			}
		}
		return false
	}
	for _, skip := range p.ExcludeSignal {
		if skip == id {
			return false
		}
	}
	if len(p.LevelFilters) > 0 {
		match := false
		for _, l := range p.LevelFilters {
			if l == int(level) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	if len(p.FamilyFilters) > 0 {
		match := false
		for _, f := range p.FamilyFilters {
			if f == family {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	return true
}

// runAssess executes the scan -> detect -> score pipeline and returns
// a populated Report. The TUI flow needs the underlying RepoIndex too
// (for the fix-apply path), so runAssessWithIndex is the lower-level
// variant; runAssess preserves the simple Report-only return for
// existing CLI callers.
func runAssess(ctx context.Context, path string, opts pipelineOptions) (acmm.Report, error) {
	rpt, _, err := runAssessWithIndex(ctx, path, opts)
	return rpt, err
}

func runAssessWithIndex(ctx context.Context, path string, opts pipelineOptions) (acmm.Report, *scanner.RepoIndex, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return acmm.Report{}, nil, err
	}

	scanStart := time.Now()
	idx, err := scanner.Scan(abs)
	if err != nil {
		return acmm.Report{}, nil, err
	}

	registered := signals.Default.All()
	if opts.Debug {
		fmt.Fprintf(opts.DebugStderr, "[debug] scanned %s: %d files, %d workflows\n",
			abs, len(idx.Files), len(idx.Workflows))
	}

	if opts.Events != nil {
		opts.Events.ScanStart(abs, len(registered))
	}

	results := make([]acmm.SignalResult, 0, len(registered))
	for _, s := range registered {
		if !opts.shouldRun(s.ID(), s.Level(), s.Family()) {
			continue
		}
		if opts.Events != nil {
			opts.Events.SignalStart(s.ID())
		}
		t0 := time.Now()
		r := s.Detect(ctx, idx)
		dur := time.Since(t0)

		entry := acmm.SignalResult{
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
			FixHint:    r.FixHint,
			Diag:       r.Diag,
		}
		results = append(results, entry)

		if opts.Events != nil {
			opts.Events.SignalComplete(entry, dur.Milliseconds())
		}
		if opts.Debug {
			fmt.Fprintf(opts.DebugStderr, "[debug] %s: status=%s score=%v confidence=%s method=%s\n",
				s.ID(), r.Status, r.Score, r.Confidence, r.Method)
		}
	}

	verdict := scoring.Compute(results, opts.Scoring)
	if opts.Events != nil {
		opts.Events.ScanComplete(verdict.Level, time.Since(scanStart).Milliseconds())
	}

	return acmm.Report{
		Schema:           buildinfo.Schema,
		ToolVersion:      buildinfo.Version,
		SignalSetVersion: buildinfo.SignalSetVersion,
		CISystem:         "github-actions",
		Repo:             abs,
		ScannedAt:        time.Now().UTC().Format(time.RFC3339),
		Verdict:          verdict,
		Signals:          results,
	}, idx, nil
}

// writeReport serializes the report in the requested format and writes
// it to outPath ("-" = stdout).
func writeReport(r acmm.Report, format, outPath string, stdout io.Writer) error {
	var data []byte
	switch format {
	case "markdown":
		data = report.Markdown(r)
	case "json":
		var err error
		data, err = json.MarshalIndent(r, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, '\n')
	case "sarif":
		// SARIF stub: minimal valid SARIF 2.1.0 envelope so CI tools
		// don't fail to parse. Full conversion lands later.
		data = []byte(`{"version":"2.1.0","$schema":"https://json.schemastore.org/sarif-2.1.0.json","runs":[]}` + "\n")
	default:
		return fmt.Errorf("unknown report format: %s", format)
	}

	if outPath == "-" || outPath == "" {
		_, err := stdout.Write(data)
		return err
	}
	return os.WriteFile(outPath, data, 0o644)
}

// stdoutIsTerminal reports whether the given writer is os.Stdout
// pointing at an interactive terminal. Tests pass *bytes.Buffer (not a
// terminal) so they take the CLI branch automatically.
func stdoutIsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
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
