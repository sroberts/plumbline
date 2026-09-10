package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sroberts/plumbline/internal/ciworkflow"
	"github.com/sroberts/plumbline/internal/fix"
)

func newInstallCICmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		apply      bool
		variant    string
		failBelow  int
		badgePath  string
		badgeLabel string
		outPath    string
		list       bool
	)

	cmd := &cobra.Command{
		Use:   "install-ci [path]",
		Short: "Scaffold the GitHub Actions workflow that runs plumbline in CI",
		Long: `plumbline install-ci — write the GitHub Actions workflow that runs
plumbline against this repo.

Installs to .github/workflows/plumbline.yml by default. Three variants
(--variant):

  full    Gate + badge      fail below a level, and keep the badge current
  gate    Maturity gate     fail the build below a minimum ACMM level
  badge   Badge drift gate  regenerate the README badge, fail if stale

The badge variants pair with 'plumbline badge', which renders a
self-contained SVG you commit and reference from the README:

  ![ACMM level](.plumbline-badge.svg)

Because the badge is byte-stable for an unchanged verdict, the workflow
can simply regenerate it and fail on a diff — the same drift-gate shape
plumbline uses for its own .plumbline.toon. A stale badge then shows up
as a reviewable change in the PR that caused it, rather than a claim
about the repo that quietly stopped being true.

If you customize the badge with --badge-label, pass the same value here:
the workflow regenerates the badge before diffing it, so a label mismatch
fails the gate on every run.

--fail-below sets the gate floor (2-5). Omit it (or pass 0) to install
the measurement without the enforcement — worth doing first in a repo
that is not yet at the level it wants, so the workflow isn't red on the
day it lands.

Default is dry-run; --apply is required to actually write. The workflow
is created, never overwritten: an existing file at the target path is an
error, so pass --out to install alongside it.

The TUI surfaces the same picker: bare 'plumbline' on a terminal shows
[w] install CI workflow.

Examples:
  # See what would be written (dry run).
  plumbline install-ci

  # Install the full workflow with an L3 floor.
  plumbline install-ci --apply --fail-below 3

  # Badge only, no gate.
  plumbline install-ci --variant badge --apply

  # Install into a specific repo, under a different filename.
  plumbline install-ci /path/to/repo --out .github/workflows/acmm.yml --apply

  # Just list the variants.
  plumbline install-ci --list

Exit codes:
  0  installed (or dry-run / --list completed)
  2  could not run (existing file, path bad, unknown variant, bad --fail-below)
  3  configuration error

See also:
  plumbline badge         render the SVG the badge variants keep current
  plumbline help ci       wiring plumbline into CI, including the composite action
  plumbline help fix      safety guarantees for plumbline-managed writes`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				printVariantList(stdout)
				return nil
			}

			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			root, err := filepath.Abs(path)
			if err != nil {
				return errCannotRun(err)
			}

			plan, err := ciworkflow.NewPlan(ciworkflow.Options{
				Variant:    variant,
				FailBelow:  failBelow,
				BadgePath:  badgePath,
				BadgeLabel: badgeLabel,
				Path:       outPath,
			})
			if err != nil {
				return errCannotRun(err)
			}

			res, err := fix.Apply(root, plan, fix.Options{DryRun: !apply})
			if err != nil {
				return errCannotRun(err)
			}

			emitFixText(stdout, plan, res, root, !apply)

			// The badge variants gate a file that does not exist yet; the
			// first CI run fails on the missing badge rather than on real
			// drift. Say so at install time.
			if apply && wantsBadge(variant) {
				b := badgeOrDefault(badgePath)
				l := badgeLabel
				if l == "" {
					l = ciworkflow.DefaultBadgeLabel
				}
				fmt.Fprintf(stderr,
					"Next: generate the badge and commit it, or the workflow's drift gate fails:\n"+
						"  plumbline badge --label %q --out %s .\n"+
						"  git add %s\n"+
						"  # then reference it from README.md:\n"+
						"  ![%s level](%s)\n",
					l, b, b, l, b)
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&apply, "apply", false, "Actually write the workflow (default is dry-run).")
	f.StringVar(&variant, "variant", ciworkflow.VariantFull,
		fmt.Sprintf("Workflow shape. One of: %s.", strings.Join(ciworkflow.IDs(), ", ")))
	f.IntVar(&failBelow, "fail-below", 0, "Gate floor for the workflow: fail the build below level N (2-5). 0 = install with no gate.")
	f.StringVar(&badgePath, "badge", ciworkflow.DefaultBadgePath, "Committed badge path the badge variants keep current.")
	f.StringVar(&badgeLabel, "badge-label", ciworkflow.DefaultBadgeLabel,
		"Badge label the workflow regenerates with. Must match the label the committed badge was built with, or the drift gate never passes.")
	f.StringVar(&outPath, "out", ciworkflow.DefaultPath, "Workflow path to write, relative to the repo root.")
	f.BoolVar(&list, "list", false, "List available workflow variants and exit.")
	return cmd
}

func wantsBadge(variant string) bool {
	return variant == "" || variant == ciworkflow.VariantFull || variant == ciworkflow.VariantBadge
}

func badgeOrDefault(p string) string {
	if p == "" {
		return ciworkflow.DefaultBadgePath
	}
	return p
}

// printVariantList prints the installable workflow variants as plain
// text, in a stable column layout for scripting.
func printVariantList(w io.Writer) {
	fmt.Fprintln(w, "Available install-ci variants:")
	fmt.Fprintln(w)
	for _, v := range ciworkflow.Variants() {
		fmt.Fprintf(w, "  %-7s %-18s %s\n", v.ID, v.Name, v.Desc)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Hint: plumbline install-ci --variant <id> --fail-below 3 --apply")
}
