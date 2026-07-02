package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/sroberts/plumbline/internal/report"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// newDiffCmd builds the `diff` subcommand: compare two snapshot artifacts
// and report how the verdict level moved and which signals changed
// status. It reads the artifacts `snapshot` writes (TOON / JSON / YAML),
// so a CI job can diff a committed `.plumbline.toon` against a freshly
// generated one without re-running the full assessment on the base.
func newDiffCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		asJSON bool
		format string
	)

	cmd := &cobra.Command{
		Use:   "diff <old> <new>",
		Short: "Compare two maturity snapshots and report the delta",
		Long: `plumbline diff — compare two maturity-snapshot artifacts.

Reads two reports written by 'plumbline snapshot' (or 'assess --report')
and prints how the verdict moved and which signals changed status. The
format of each input is inferred from its extension (.toon / .json /
.yaml); use "-" to read one from stdin and --format to force the format.

This is the cheap way to compute a maturity delta in CI: diff the
committed .plumbline.toon at the base against a fresh snapshot of the
head, instead of re-assessing both sides.

Examples:
  # Base (committed) vs head (fresh), as a PR-comment-ready summary.
  git show origin/main:.plumbline.toon > /tmp/base.toon
  plumbline snapshot --out /tmp/head.toon .
  plumbline diff /tmp/base.toon /tmp/head.toon

  # Machine-readable delta.
  plumbline diff base.toon head.toon --json

  # Force a format when reading from stdin.
  plumbline snapshot --format json --out - . | plumbline diff base.json - --format json

Exit codes:
  0  delta computed (whether or not anything changed)
  2  could not run (unreadable/undecodable artifact)

See also:
  plumbline snapshot         write the artifact this command consumes
  plumbline assess           run the full assessment`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldR, err := loadReport(cmd, args[0], format)
			if err != nil {
				return errCannotRun(err)
			}
			newR, err := loadReport(cmd, args[1], format)
			if err != nil {
				return errCannotRun(err)
			}

			delta := report.Diff(oldR, newR)

			if asJSON {
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				if err := enc.Encode(delta); err != nil {
					return errCannotRun(err)
				}
				return nil
			}
			_, err = stdout.Write(report.RenderDeltaMarkdown(delta))
			return err
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Emit the delta as JSON instead of a Markdown summary.")
	cmd.Flags().StringVar(&format, "format", "", "Force input format (toon|json|yaml) instead of inferring from the file extension.")
	return cmd
}

// loadReport reads and decodes a snapshot artifact from path ("-" =
// stdin). When format is empty it is inferred from the path extension.
func loadReport(cmd *cobra.Command, path, format string) (acmm.Report, error) {
	data, err := readArtifact(cmd, path)
	if err != nil {
		return acmm.Report{}, err
	}
	fmtName := format
	if fmtName == "" {
		fmtName = report.FormatFromPath(path)
	}
	return report.DecodeReport(data, fmtName)
}

func readArtifact(cmd *cobra.Command, path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(cmd.InOrStdin())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}
