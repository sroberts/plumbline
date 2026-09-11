package main

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sroberts/plumbline/internal/dependabot"
	"github.com/sroberts/plumbline/internal/fix"
	"github.com/sroberts/plumbline/internal/scanner"
)

func newInstallDependabotCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		apply    bool
		interval string
		list     bool
	)

	cmd := &cobra.Command{
		Use:   "install-dependabot [path]",
		Short: "Scaffold .github/dependabot.yml from the manifests this repo actually has",
		Long: `plumbline install-dependabot — write a Dependabot config for this repo.

Scans for dependency manifests and writes one update block per manifest
found, at .github/dependabot.yml. The content is derived, not templated:
a config naming an ecosystem whose manifest is absent makes Dependabot
error on every run, and one that omits a manifest silently never checks
it.

GitHub Actions is included whenever the repo has workflows. Pinned action
versions go stale exactly like any other dependency, and that is the case
most repos miss — a runtime deprecation takes every workflow out at once,
with no manifest anywhere to warn you first.

Recognized: gomod, npm, cargo, pip, bundler, composer, maven, gradle,
docker, terraform, github-actions.

Default is dry-run; --apply is required to write. The file is created,
never overwritten: an existing config encodes deliberate choices (ignores,
groups, reviewers) that are not plumbline's to replace.

This command sits OUTSIDE the assessment loop, deliberately. plumbline's
remedies normally reach you through assess -> next_gap -> fix/fix_hint;
this one does not appear there, no signal detects the file it writes, and
running it moves no verdict. It is dependency hygiene, which is worth
having and is not a maturity level.

Two reasons it stays outside. A signal that credited the config would be
plumbline scoring a file plumbline wrote — the circularity SPEC.md §4
exists to prevent. And the loop that would genuinely rate (updates
merging without a human, gated on CI) is one this command does not
write: it emits update blocks and a schedule, nothing that merges.

Examples:
  # See what would be written for this repo.
  plumbline install-dependabot

  # Write it.
  plumbline install-dependabot --apply

  # Monthly instead of weekly.
  plumbline install-dependabot --interval monthly --apply

  # Just show what was detected.
  plumbline install-dependabot --list

Exit codes:
  0  installed (or dry-run / --list completed)
  2  could not run (existing config, no manifests found, bad interval)
  3  configuration error

See also:
  plumbline install-ci       the workflow that runs plumbline in CI
  plumbline help fix         safety guarantees for plumbline-managed writes`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			root, err := filepath.Abs(path)
			if err != nil {
				return errCannotRun(err)
			}
			idx, err := scanner.Scan(root)
			if err != nil {
				return errCannotRun(err)
			}
			updates := dependabot.Detect(idx)

			if list {
				if len(updates) == 0 {
					fmt.Fprintln(stdout, "No dependency manifests found.")
					return nil
				}
				fmt.Fprintln(stdout, "Detected ecosystems:")
				fmt.Fprintln(stdout)
				for _, u := range updates {
					fmt.Fprintf(stdout, "  %-16s %s\n", u.Ecosystem, u.Directory)
				}
				fmt.Fprintln(stdout)
				fmt.Fprintln(stdout, "Hint: plumbline install-dependabot --apply")
				return nil
			}

			plan, err := dependabot.NewPlan(updates, dependabot.Options{Interval: interval})
			if err != nil {
				return errCannotRun(err)
			}
			res, err := fix.Apply(root, plan, fix.Options{DryRun: !apply})
			if err != nil {
				return errCannotRun(err)
			}
			emitFixText(stdout, plan, res, root, !apply)
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&apply, "apply", false, "Actually write the config (default is dry-run).")
	f.StringVar(&interval, "interval", dependabot.DefaultInterval, "Update cadence: daily|weekly|monthly.")
	f.BoolVar(&list, "list", false, "List the ecosystems detected in this repo and exit.")
	return cmd
}
