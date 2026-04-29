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

// assessFlags holds every flag the assess pipeline understands. Both
// the `assess` subcommand and the root command bind the same struct so
// `plumbline [path]` and `plumbline assess [path]` are at flag parity.
type assessFlags struct {
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
}

// bindAssessFlags registers the assess flag set on cmd. Used by the
// assess subcommand and by the root command's default action.
func bindAssessFlags(cmd *cobra.Command, f *assessFlags) {
	fs := cmd.Flags()
	fs.BoolVar(&f.asJSON, "json", false, "Emit JSON to stdout. Implies --cli. Schema: 'plumbline schema verdict'.")
	fs.StringVar(&f.reportFmt, "report", "", "Write a report file. fmt: markdown|json|sarif. Implies --cli.")
	fs.StringVar(&f.outPath, "out", "-", "Report destination. \"-\" = stdout.")
	fs.StringVar(&f.eventsFmt, "events", "", "Stream progress events to stderr. fmt: ndjson|text. Schema: 'plumbline schema event'.")
	fs.BoolVarP(&f.quiet, "quiet", "q", false, "Suppress banners, progress, and trailing hints. Implies --cli.")
	fs.BoolVar(&f.noColor, "no-color", false, "Disable ANSI color (also via NO_COLOR=1).")
	fs.BoolVar(&f.cli, "cli", false, "Force pure-CLI mode. Auto-set when stdout is not a TTY.")
	fs.BoolVar(&f.forceTUI, "tui", false, "Force TUI even when stdout is not a TTY.")
	fs.IntVar(&f.failBelow, "fail-below", 0, "Exit 1 if assessed level < N (2-5). 0 = no gate.")
	fs.StringVar(&f.profile, "profile", "default", "Named signal preset. See 'plumbline help profiles'.")
	fs.StringVar(&f.configPath, "config", "", "Override config path. Default: .plumbline.yml.")
	fs.StringVar(&f.minConfidence, "min-confidence", "low", "Minimum confidence to credit a signal: low|medium|high.")
	fs.StringVar(&f.signalSet, "signal-set", "latest", "Pin signal rule-set version (e.g., v1).")
	fs.StringVar(&f.ciSystem, "ci-system", "auto", "CI flavor: auto|github-actions. Other systems are M4+.")
	fs.BoolVar(&f.debug, "debug", false, "Emit detection diagnostics on stderr. Implies --cli.")
	fs.StringVar(&f.clock, "clock", "wall", "Timestamp source for events: wall|fixed|relative.")
	fs.StringSliceVar(&f.includeSignal, "include-signal", nil, "Only run the listed signals. Repeatable.")
	fs.StringSliceVar(&f.excludeSignal, "exclude-signal", nil, "Skip the listed signals. Repeatable.")
	fs.IntSliceVar(&f.levelFilters, "level", nil, "Only run signals at level N (2-5). Repeatable.")
	fs.StringSliceVar(&f.familyFilters, "family", nil, "Only run signals in family <name>. Repeatable.")
}

// makeAssessRunE returns a cobra RunE closure that drives the assess
// pipeline using the bound flags + provided I/O. Used by both the
// assess subcommand and the root command.
func makeAssessRunE(flags *assessFlags, stdout, stderr io.Writer) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if flags.cli && flags.forceTUI {
			return errCannotRun(errors.New("--cli and --tui are mutually exclusive"))
		}
		if len(flags.includeSignal) > 0 && len(flags.excludeSignal) > 0 {
			return errCannotRun(errors.New("--include-signal and --exclude-signal are mutually exclusive"))
		}
		if flags.reportFmt != "" && flags.reportFmt != "json" && flags.reportFmt != "markdown" && flags.reportFmt != "sarif" {
			return errCannotRun(fmt.Errorf("invalid --report %q (want json|markdown|sarif)", flags.reportFmt))
		}
		if flags.eventsFmt != "" && flags.eventsFmt != "ndjson" && flags.eventsFmt != "text" {
			return errCannotRun(fmt.Errorf("invalid --events %q (want ndjson|text)", flags.eventsFmt))
		}

		path := "."
		if len(args) == 1 {
			path = args[0]
		}

		confLevel, err := parseConfidence(flags.minConfidence)
		if err != nil {
			return errCannotRun(err)
		}

		emitter := report.NewEventEmitter(stderr, flags.eventsFmt == "ndjson")

		pipelineOpts := pipelineOptions{
			IncludeSignal: flags.includeSignal,
			ExcludeSignal: flags.excludeSignal,
			LevelFilters:  flags.levelFilters,
			FamilyFilters: flags.familyFilters,
			Scoring:       scoring.Options{MinConfidence: confLevel},
			Events:        emitter,
			Debug:         flags.debug,
			DebugStderr:   stderr,
		}

		// Mode selection per SPEC.md §4.
		modeIsTUI := flags.forceTUI ||
			(!flags.cli && !flags.asJSON && flags.reportFmt == "" &&
				flags.eventsFmt == "" && !flags.quiet && !flags.debug &&
				stdoutIsTerminal(stdout))

		if modeIsTUI {
			rpt, err := tui.Run(func(ctx context.Context) (acmm.Report, *scanner.RepoIndex, error) {
				return runAssessWithIndex(ctx, path, pipelineOpts)
			})
			if err != nil {
				return errCannotRun(err)
			}
			if flags.failBelow > 0 && int(rpt.Verdict.Level) < flags.failBelow {
				return &exitError{code: exitGateFailed, err: fmt.Errorf("verdict level %d is below --fail-below %d", rpt.Verdict.Level, flags.failBelow)}
			}
			return nil
		}

		rpt, err := runAssess(cmd.Context(), path, pipelineOpts)
		if err != nil {
			return errCannotRun(err)
		}

		switch {
		case flags.reportFmt != "":
			if err := writeReport(rpt, flags.reportFmt, flags.outPath, stdout); err != nil {
				return errCannotRun(err)
			}
		case flags.asJSON:
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(rpt); err != nil {
				return errCannotRun(err)
			}
		case !flags.quiet:
			writeBriefText(stdout, rpt)
		}

		if flags.failBelow > 0 && int(rpt.Verdict.Level) < flags.failBelow {
			return &exitError{
				code: exitGateFailed,
				err:  fmt.Errorf("verdict level %d is below --fail-below %d", rpt.Verdict.Level, flags.failBelow),
			}
		}
		return nil
	}
}

func newAssessCmd(stdout, stderr io.Writer) *cobra.Command {
	flags := &assessFlags{}

	cmd := &cobra.Command{
		Use:   "assess [path]",
		Short: "Scan a repo and report its ACMM maturity level",
		Long: `plumbline assess — scan a repo and report its ACMM maturity level.

Walks the repository at [path] (default ".") and runs every enabled
signal detector against it. Produces a verdict (level 1-5), per-level
scores, and the list of signals that would unlock the next level.

Mode is auto-detected: TUI on a terminal, CLI when piped or in CI.
Use --cli to force non-interactive output, --tui to force the TUI.

This command is also the default — running 'plumbline [path]' without
a subcommand is equivalent to 'plumbline assess [path]'.

Examples:
  # Bare invocation: scan the current directory (TUI on a terminal).
  plumbline

  # Same, explicit subcommand.
  plumbline assess

  # Machine-readable, with progress events on stderr.
  plumbline --json --events ndjson 2>events.log >verdict.json

  # CI gate: fail if not at level 3 or higher.
  plumbline --fail-below 3 --quiet

  # Scan a specific repo, save markdown report.
  plumbline /path/to/repo --report markdown --out maturity.md

  # Only run the L3 coverage signals.
  plumbline --level 3 --family coverage --json

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
		RunE: makeAssessRunE(flags, stdout, stderr),
	}
	bindAssessFlags(cmd, flags)
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
		var err error
		data, err = report.SARIF(r)
		if err != nil {
			return err
		}
		data = append(data, '\n')
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

// writeBriefText prints a one-screen summary of the verdict.
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
