package main

import (
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sroberts/plumbline/internal/fix"
	"github.com/sroberts/plumbline/internal/skill"
)

func newInstallSkillCmd(stdout, stderr io.Writer) *cobra.Command {
	var apply bool

	cmd := &cobra.Command{
		Use:   "install-skill [path]",
		Short: "Install a Claude Code skill so AI agents in this repo know how to drive plumbline",
		Long: `plumbline install-skill — write a Claude Code skill into the repo at
.claude/skills/plumbline/SKILL.md.

Once installed, Claude Code (and any harness that reads .claude/skills/)
will know when to invoke plumbline and what its stable interfaces are
(commands, JSON schemas, exit codes, fix-apply safety guards). The
skill is the canonical "how to use plumbline from an AI agent" guide.

Default is dry-run; --apply is required to actually write. Refuses to
overwrite an existing SKILL.md so user customizations are preserved
(remove the existing file manually first if you want to reinstall).

The TUI surfaces the same install action: bare 'plumbline' on a
terminal shows an [i] install skill hint when the skill isn't already
present in the scanned repo.

Examples:
  # See what would be written.
  plumbline install-skill

  # Install in the current repo.
  plumbline install-skill --apply

  # Install in a different repo.
  plumbline install-skill /path/to/repo --apply

Exit codes:
  0  installed (or dry-run completed)
  2  could not run (existing skill, path bad, etc.)
  3  configuration error

See also:
  plumbline help fix      safety guarantees for plumbline-managed writes
  plumbline help agents   the same guidance as the skill, but as topical help`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				return errCannotRun(err)
			}

			plan := skill.NewPlan()
			res, err := fix.Apply(abs, plan, fix.Options{DryRun: !apply})
			if err != nil {
				return errCannotRun(err)
			}

			emitFixText(stdout, plan, res, abs, !apply)
			return nil
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually write the skill (default is dry-run).")
	return cmd
}
