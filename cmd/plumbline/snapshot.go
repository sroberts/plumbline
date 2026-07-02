package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sroberts/plumbline/internal/config"
	"github.com/sroberts/plumbline/internal/scoring"
)

// snapshotFlags holds the snapshot subcommand's flag set.
type snapshotFlags struct {
	format        string
	outPath       string
	configPath    string
	minConfidence string
}

// newSnapshotCmd builds the `snapshot` subcommand. It runs the standard
// assess pipeline and writes a committable maturity-state artifact —
// `.plumbline.toon` by default (Token-Oriented Object Notation), or JSON
// / YAML when forced with --format. Unlike `assess --report`, snapshot
// defaults the output to a repo-root dotfile so `plumbline snapshot` with
// no flags produces the artifact teams commit and diff over time.
func newSnapshotCmd(stdout, stderr io.Writer) *cobra.Command {
	flags := &snapshotFlags{}

	cmd := &cobra.Command{
		Use:   "snapshot [path]",
		Short: "Write a committable maturity-state artifact (.plumbline.toon)",
		Long: `plumbline snapshot — write a maturity-state artifact for a repo.

Runs the same scan + score pipeline as 'assess' and serializes the full
report to a file. The default format is TOON (Token-Oriented Object
Notation) written to .plumbline.toon — a compact, diff-friendly artifact
meant to be committed and tracked over time. Force json or yaml with
--format for tools that prefer them.

The artifact is a lossless re-encoding of 'assess --json': same fields,
same values, different notation. Signals disabled in .plumbline.yml are
honored, exactly as in a normal assess.

Examples:
  # Write .plumbline.toon for the current repo.
  plumbline snapshot

  # Force JSON, custom path.
  plumbline snapshot --format json --out docs/maturity.json

  # Emit YAML to stdout (e.g. to pipe elsewhere).
  plumbline snapshot --format yaml --out -

Exit codes:
  0  artifact written
  2  could not run (path not a directory, IO error)
  3  configuration error

See also:
  plumbline assess           full assess pipeline and other report formats
  plumbline schema verdict   the JSON schema the artifact conforms to`,
		Args: cobra.MaximumNArgs(1),
		RunE: makeSnapshotRunE(flags, stdout, stderr),
	}

	fs := cmd.Flags()
	fs.StringVar(&flags.format, "format", "toon", "Artifact format: toon|json|yaml.")
	fs.StringVar(&flags.outPath, "out", "", "Output path. Default: .plumbline.<ext>. \"-\" = stdout.")
	fs.StringVar(&flags.configPath, "config", "", "Override config path. Default: .plumbline.yml.")
	fs.StringVar(&flags.minConfidence, "min-confidence", "low", "Minimum confidence to credit a signal: low|medium|high.")
	return cmd
}

func makeSnapshotRunE(flags *snapshotFlags, stdout, stderr io.Writer) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		format := strings.ToLower(flags.format)
		switch format {
		case "toon", "json", "yaml":
		default:
			return errCannotRun(fmt.Errorf("invalid --format %q (want toon|json|yaml)", format))
		}

		path := "."
		if len(args) == 1 {
			path = args[0]
		}

		confLevel, err := parseConfidence(flags.minConfidence)
		if err != nil {
			return errCannotRun(err)
		}

		// Load .plumbline.yml. Explicit --config errors on a missing
		// file; the default lookup is optional and returns nil if absent.
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

		rpt, err := runAssess(cmd.Context(), path, pipelineOptions{
			ExcludeSignal: exclude,
			Scoring:       scoringOpts,
		})
		if err != nil {
			return errCannotRun(err)
		}

		// With no explicit --out, drop the dotfile inside the scanned repo
		// so `plumbline snapshot /some/repo` writes /some/repo/.plumbline.toon
		// rather than a file in the caller's working directory. An explicit
		// --out (including "-") is honored verbatim, relative to the CWD.
		outPath := flags.outPath
		if outPath == "" {
			outPath = filepath.Join(path, defaultSnapshotName(format))
		}
		if err := writeReport(rpt, format, outPath, stdout); err != nil {
			return errCannotRun(err)
		}

		// Confirm to stderr so stdout stays clean when writing a file;
		// suppress it when the artifact itself went to stdout.
		if outPath != "-" {
			fmt.Fprintf(stderr, "wrote %s (level %d — %s)\n", outPath, rpt.Verdict.Level, rpt.Verdict.Name)
		}
		return nil
	}
}

// defaultSnapshotName maps a format to its default artifact filename.
func defaultSnapshotName(format string) string {
	switch format {
	case "json":
		return ".plumbline.json"
	case "yaml":
		return ".plumbline.yaml"
	default:
		return ".plumbline.toon"
	}
}
