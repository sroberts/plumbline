package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sroberts/plumbline/internal/badge"
	"github.com/sroberts/plumbline/internal/config"
	"github.com/sroberts/plumbline/internal/report"
	"github.com/sroberts/plumbline/internal/scoring"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// defaultBadgeName is the repo-root file `badge` writes with no --out.
// A dotfile keeps it out of the way of the repo's own assets while
// staying next to the README that references it.
const defaultBadgeName = ".plumbline-badge.svg"

// badgeFlags holds the badge subcommand's flag set.
type badgeFlags struct {
	outPath       string
	label         string
	from          string
	configPath    string
	minConfidence string
}

// newBadgeCmd builds the `badge` subcommand — render the verdict as a
// committable SVG status badge for the README.
func newBadgeCmd(stdout, stderr io.Writer) *cobra.Command {
	flags := &badgeFlags{}

	cmd := &cobra.Command{
		Use:   "badge [path]",
		Short: "Render the ACMM verdict as a committable SVG status badge",
		Long: `plumbline badge — render the verdict as an SVG status badge.

Runs the same scan + score pipeline as 'assess' and writes a
self-contained SVG to .plumbline-badge.svg, ready to reference from the
README with a relative path:

  ![ACMM level](.plumbline-badge.svg)

The badge is self-hosted on purpose. plumbline makes no network calls,
and a committed SVG keeps that property for the reader: it renders in
private repos, behind a proxy, and offline, with no third-party badge
service in the path. The trade is that the file goes stale when the
verdict moves — guard it with the CI drift gate below.

Output is byte-stable for an unchanged repo, so regenerate-and-diff is
a valid gate:

  plumbline badge --out .plumbline-badge.svg .
  git diff --exit-code -- .plumbline-badge.svg

Signals disabled in .plumbline.yml are honored, exactly as in a normal
assess, so the badge can never disagree with the gate beside it.

--from renders from an artifact 'plumbline snapshot' already wrote,
instead of rescanning — the cheap path for a workflow that has one.

Examples:
  # Write .plumbline-badge.svg for the current repo.
  plumbline badge

  # Stream the SVG instead of writing a file.
  plumbline badge --out - > badge.svg

  # Rename the left-hand side.
  plumbline badge --label "AI readiness"

  # Render from a committed snapshot without rescanning.
  plumbline badge --from .plumbline.toon

Exit codes:
  0  badge written
  2  could not run (path not a directory, unreadable artifact, IO error)
  3  configuration error

See also:
  plumbline install-ci       scaffold the workflow that regenerates this badge
  plumbline snapshot         write a committable .plumbline.toon artifact
  plumbline help ci          wiring plumbline into CI`,
		Args: cobra.MaximumNArgs(1),
		RunE: makeBadgeRunE(flags, stdout, stderr),
	}

	fs := cmd.Flags()
	fs.StringVar(&flags.outPath, "out", "", "Output path. Default: "+defaultBadgeName+" in the scanned repo. \"-\" = stdout.")
	fs.StringVar(&flags.label, "label", badge.DefaultLabel, "Left-hand badge text.")
	fs.StringVar(&flags.from, "from", "", "Render from an existing snapshot artifact (.toon/.json/.yaml) instead of scanning.")
	fs.StringVar(&flags.configPath, "config", "", "Override config path. Default: .plumbline.yml.")
	fs.StringVar(&flags.minConfidence, "min-confidence", "low", "Minimum confidence to credit a signal: low|medium|high.")
	return cmd
}

func makeBadgeRunE(flags *badgeFlags, stdout, stderr io.Writer) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) == 1 {
			if flags.from != "" {
				// Both name a repo, and they can disagree. Refusing is
				// kinder than silently picking one.
				return errCannotRun(errors.New("--from and a path argument are mutually exclusive: --from already names the assessed repo"))
			}
			path = args[0]
		}

		// --from short-circuits the scan entirely: the artifact already
		// holds a scored verdict, so re-deriving it would risk the badge
		// and the committed snapshot disagreeing.
		if flags.from != "" {
			data, err := os.ReadFile(flags.from)
			if err != nil {
				return errCannotRun(fmt.Errorf("read artifact: %w", err))
			}
			decoded, err := report.DecodeReport(data, report.FormatFromPath(flags.from))
			if err != nil {
				return errCannotRun(fmt.Errorf("decode %s: %w", flags.from, err))
			}
			return writeBadge(
				badge.SVG(decoded.Verdict, badge.Options{Label: flags.label}),
				badgeOutPath(flags.outPath, filepath.Dir(flags.from)),
				stdout, stderr, decoded.Verdict)
		}

		confLevel, err := parseConfidence(flags.minConfidence)
		if err != nil {
			return errCannotRun(err)
		}

		var cfg *config.Config
		if flags.configPath != "" {
			cfg, err = config.Load(flags.configPath)
		} else {
			cfg, err = config.LoadDefault(path)
		}
		if err != nil {
			return errCannotRun(err)
		}

		scoringOpts := scoring.Options{MinConfidence: confLevel}
		var exclude []string
		if cfg != nil {
			if cfg.Thresholds != nil && cfg.Thresholds.Pass > 0 {
				scoringOpts.PassThreshold = cfg.Thresholds.Pass
			}
			exclude = cfg.DisabledSignals()
		}

		assessed, err := runAssess(cmd.Context(), path, pipelineOptions{
			ExcludeSignal: exclude,
			Scoring:       scoringOpts,
		})
		if err != nil {
			return errCannotRun(err)
		}

		return writeBadge(
			badge.SVG(assessed.Verdict, badge.Options{Label: flags.label}),
			badgeOutPath(flags.outPath, path),
			stdout, stderr, assessed.Verdict)
	}
}

// badgeOutPath resolves --out against the repo being assessed, so
// `plumbline badge /some/repo` drops the SVG beside that repo's README
// rather than in the caller's working directory. An explicit --out
// (including "-") is honored verbatim, relative to the CWD.
func badgeOutPath(out, repo string) string {
	if out == "" {
		return filepath.Join(repo, defaultBadgeName)
	}
	return out
}

// writeBadge emits the SVG and confirms on stderr, keeping stdout clean
// for the `--out -` pipe case.
func writeBadge(svg []byte, outPath string, stdout, stderr io.Writer, v acmm.Verdict) error {
	if outPath == "-" {
		if _, err := stdout.Write(svg); err != nil {
			return errCannotRun(err)
		}
		return nil
	}
	// Every other writer in the tool goes through internal/fix, which
	// refuses to clobber. badge has to overwrite — regenerating in place
	// is the whole workflow — so it draws the line at files it did not
	// write: `--out README.md` is a typo, not an instruction.
	if existing, err := os.ReadFile(outPath); err == nil && !badge.IsGenerated(existing) {
		return errCannotRun(fmt.Errorf(
			"refusing to overwrite %s: it is not a plumbline-generated badge (remove it first if you meant to replace it)",
			outPath))
	}
	if err := os.WriteFile(outPath, svg, 0o644); err != nil {
		return errCannotRun(err)
	}
	fmt.Fprintf(stderr, "wrote %s (level %d — %s)\n", outPath, v.Level, v.Name)
	return nil
}
