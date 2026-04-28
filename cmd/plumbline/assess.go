package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
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
			_ = args
			// Cross-flag validation that's true regardless of milestone.
			if cli && tui {
				return errCannotRun(errors.New("--cli and --tui are mutually exclusive"))
			}
			if len(includeSignal) > 0 && len(excludeSignal) > 0 {
				return errCannotRun(errors.New("--include-signal and --exclude-signal are mutually exclusive"))
			}
			fmt.Fprintln(stderr, "(M1: assess is wired up but the scanner and signals land in the next PR)")
			fmt.Fprintln(stderr, "Hint: run 'plumbline help' for the topic index.")
			return errCannotRun(errors.New("not implemented in this milestone"))
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
