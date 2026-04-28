package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sroberts/plumbline/internal/fix"
	"github.com/sroberts/plumbline/internal/skill"
	"github.com/sroberts/plumbline/pkg/acmm"
)

func newInstallSkillCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		apply  bool
		target string
		list   bool
		global bool
	)

	cmd := &cobra.Command{
		Use:   "install-skill [path]",
		Short: "Install plumbline's usage guide for a coding-agent tool (claude/cursor/codex/etc.)",
		Long: `plumbline install-skill — write plumbline's usage guide into the
location expected by the chosen coding-agent tool.

Supported targets (--target):

  claude    Claude Code             .claude/skills/plumbline/SKILL.md
  cursor    Cursor                  .cursor/rules/plumbline.mdc
  gemini    Gemini Code Assist      GEMINI.md                       (shared)
  codex     OpenAI Codex CLI        AGENTS.md                       (shared)
  opencode  OpenCode                AGENTS.md                       (shared)
  windsurf  Windsurf                .windsurfrules                  (shared)
  cline     Cline                   .clinerules                     (shared)
  copilot   GitHub Copilot          .github/copilot-instructions.md (shared)

"Shared" targets write to a file the user might already be using for
other purposes. The fix-apply pipeline refuses to overwrite, so you'll
get a clear error if the file exists; remove or merge manually.

Scope: project-local by default (the file lands in the current repo).
Pass --global to install at user scope under $HOME instead — useful
when you want plumbline guidance available across every repo. Not all
targets have a documented global location; --global on those errors
with a helpful list.

Default is dry-run; --apply is required to actually write. Use --list
to print just the available targets.

The TUI surfaces the same picker: bare 'plumbline' on a terminal shows
[i] install skill which opens an interactive target selector.

Examples:
  # See what would be written for the default target (claude).
  plumbline install-skill

  # Install for Cursor in this repo.
  plumbline install-skill --target cursor --apply

  # Install for Codex CLI in a specific repo.
  plumbline install-skill /path/to/repo --target codex --apply

  # Install Gemini guidance globally (~/.gemini/GEMINI.md).
  plumbline install-skill --target gemini --global --apply

  # Just list the targets.
  plumbline install-skill --list

Exit codes:
  0  installed (or dry-run / --list completed)
  2  could not run (existing file, path bad, unknown target)
  3  configuration error

See also:
  plumbline help fix      safety guarantees for plumbline-managed writes
  plumbline help agents   the same guidance as the skill, but as topical help`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				printTargetList(stdout)
				return nil
			}
			if target == "" {
				target = "claude"
			}
			if global && len(args) > 0 {
				return errCannotRun(fmt.Errorf("--global installs to ~/, not a repo path; do not pass [path]"))
			}

			t, ok := skill.TargetByID(target)
			if !ok {
				return errCannotRun(fmt.Errorf("unknown install target %q (available: %s)",
					target, strings.Join(skill.IDs(), ", ")))
			}

			var (
				root string
				plan acmm.FixPlan
				err  error
			)
			if global {
				root, err = os.UserHomeDir()
				if err != nil {
					return errCannotRun(fmt.Errorf("could not resolve user home dir: %w", err))
				}
				plan, err = skill.NewPlanForGlobal(target)
				if err != nil {
					return errCannotRun(err)
				}
			} else {
				path := "."
				if len(args) == 1 {
					path = args[0]
				}
				root, err = filepath.Abs(path)
				if err != nil {
					return errCannotRun(err)
				}
				plan, err = skill.NewPlanFor(target)
				if err != nil {
					return errCannotRun(err)
				}
			}

			if t.SharedFile && !global {
				fmt.Fprintf(stderr, "Note: %s installs to %s, which other agent tools may also read.\n",
					t.Name, t.Path)
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
	f.BoolVar(&apply, "apply", false, "Actually write the skill (default is dry-run).")
	f.StringVar(&target, "target", "", fmt.Sprintf("Coding-agent tool. One of: %s. Default: claude.", strings.Join(skill.IDs(), ", ")))
	f.BoolVar(&list, "list", false, "List available install-skill targets and exit.")
	f.BoolVar(&global, "global", false, "Install at user scope (under $HOME) instead of the current repo.")
	return cmd
}

// printTargetList prints the supported install targets as plain text.
// Stable column layout for scripting.
func printTargetList(w io.Writer) {
	fmt.Fprintln(w, "Available install-skill targets:")
	fmt.Fprintln(w)
	for _, t := range skill.Targets() {
		shared := ""
		if t.SharedFile {
			shared = " (shared file)"
		}
		fmt.Fprintf(w, "  %-10s %-20s %s%s\n", t.ID, t.Name, t.Path, shared)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Hint: plumbline install-skill --target <id> --apply")
}
