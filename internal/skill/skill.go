// Package skill holds the embedded plumbline-usage guides for various
// coding-agent tools (Claude Code, Cursor, Codex CLI, etc.) and the
// FixPlan helpers that the install-skill command and the TUI consume.
//
// One Target == one (tool, file path, body) tuple. Targets() returns
// them in display order. Each tool's body is a tailored markdown
// document; the prose is shared (corePlumblineGuide) and only the
// frontmatter / preamble varies per tool.
package skill

import (
	"fmt"
	"strings"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// Target identifies one place plumbline can install its usage guide.
type Target struct {
	// ID is the stable CLI-visible identifier ("claude", "cursor", ...).
	ID string

	// Name is human-readable ("Claude Code", "Cursor", ...).
	Name string

	// Path is the repo-relative install location for project-scope
	// installs.
	Path string

	// GlobalPath is the install location for user-scope (global)
	// installs, relative to the user's home directory. Empty when
	// the tool has no documented global rules location.
	GlobalPath string

	// SharedFile is true when Path is a file the user may already be
	// using for other purposes (AGENTS.md, .windsurfrules, etc.).
	// The CLI / TUI surface a warning before refusing to overwrite.
	SharedFile bool

	// body is the full file body to write when creating fresh.
	body string
}

// SupportsGlobal reports whether this target has a documented global
// install location.
func (t Target) SupportsGlobal() bool { return t.GlobalPath != "" }

// Body returns the full file body that would be written for this target.
// Exposed so tests and the TUI preview can render it without having to
// build a FixPlan first.
func (t Target) Body() string { return t.body }

// targets is the ordered list of supported install targets. Order is
// display-friendly: dedicated-skill-directory tools first, then shared-
// file conventions.
var targets = []Target{
	{
		ID:         "claude",
		Name:       "Claude Code",
		Path:       ".claude/skills/plumbline/SKILL.md",
		GlobalPath: ".claude/skills/plumbline/SKILL.md", // resolved against $HOME
		body:       claudeSkillBody,
	},
	{
		ID:         "cursor",
		Name:       "Cursor",
		Path:       ".cursor/rules/plumbline.mdc",
		GlobalPath: ".cursor/rules/plumbline.mdc",
		body:       cursorRuleBody,
	},
	{
		ID:         "gemini",
		Name:       "Gemini Code Assist",
		Path:       "GEMINI.md",
		GlobalPath: ".gemini/GEMINI.md",
		SharedFile: true,
		body:       geminiBody,
	},
	{
		ID:         "codex",
		Name:       "OpenAI Codex CLI",
		Path:       "AGENTS.md",
		GlobalPath: ".codex/AGENTS.md",
		SharedFile: true,
		body:       agentsBody,
	},
	{
		ID:         "opencode",
		Name:       "OpenCode",
		Path:       "AGENTS.md",
		GlobalPath: ".config/opencode/AGENTS.md",
		SharedFile: true,
		body:       agentsBody,
	},
	{
		ID:         "windsurf",
		Name:       "Windsurf",
		Path:       ".windsurfrules",
		SharedFile: true,
		body:       windsurfBody,
	},
	{
		ID:         "cline",
		Name:       "Cline",
		Path:       ".clinerules",
		SharedFile: true,
		body:       windsurfBody,
	},
	{
		ID:         "copilot",
		Name:       "GitHub Copilot",
		Path:       ".github/copilot-instructions.md",
		SharedFile: true,
		body:       copilotBody,
	},
}

// Targets returns all known install targets in display order.
func Targets() []Target {
	out := make([]Target, len(targets))
	copy(out, targets)
	return out
}

// TargetByID looks up a target by its stable CLI ID.
func TargetByID(id string) (Target, bool) {
	for _, t := range targets {
		if t.ID == id {
			return t, true
		}
	}
	return Target{}, false
}

// IDs returns every target ID in display order. Used by --list and
// for error-message construction.
func IDs() []string {
	out := make([]string, len(targets))
	for i, t := range targets {
		out[i] = t.ID
	}
	return out
}

// NewPlanFor returns the install-skill FixPlan for the named target,
// installed at project scope (Path resolved against the repo root).
// Errors if id is not in the registry.
func NewPlanFor(id string) (acmm.FixPlan, error) {
	return planFor(id, false)
}

// NewPlanForGlobal returns the install-skill FixPlan for the named
// target at user scope (GlobalPath resolved against the user's home
// directory). Errors if id is not in the registry, or if the target
// has no documented global location.
//
// Note: the Op's Path is the GlobalPath as a *relative path*. The
// caller resolves it against the user's $HOME before passing the plan
// to fix.Apply, since fix.Apply rejects absolute paths and paths that
// escape the install root.
func NewPlanForGlobal(id string) (acmm.FixPlan, error) {
	return planFor(id, true)
}

func planFor(id string, global bool) (acmm.FixPlan, error) {
	t, ok := TargetByID(id)
	if !ok {
		return acmm.FixPlan{}, fmt.Errorf("unknown install target %q (available: %s)",
			id, strings.Join(IDs(), ", "))
	}
	path := t.Path
	scope := "at " + t.Path + " (in repo)"
	if global {
		if !t.SupportsGlobal() {
			return acmm.FixPlan{}, fmt.Errorf("target %q has no documented global location; install at project scope (drop --global) or use one of: %s",
				id, strings.Join(globalCapableIDs(), ", "))
		}
		path = t.GlobalPath
		scope = "at ~/" + t.GlobalPath + " (user-scope)"
	}
	return acmm.FixPlan{
		SignalID: "install-skill:" + t.ID,
		Summary:  fmt.Sprintf("Install plumbline guidance for %s %s", t.Name, scope),
		Ops: []acmm.FixOp{{
			Kind: acmm.FixCreateFile,
			Path: path,
			Body: []byte(t.body),
		}},
	}, nil
}

// globalCapableIDs returns target IDs that have a documented global
// install location, in display order.
func globalCapableIDs() []string {
	var out []string
	for _, t := range targets {
		if t.SupportsGlobal() {
			out = append(out, t.ID)
		}
	}
	return out
}

// Path is the canonical Claude Code skill path. Backward-compat for
// callers (the TUI's installed-check still uses this for the default
// target; the multi-target picker uses TargetByID).
const Path = ".claude/skills/plumbline/SKILL.md"

// NewPlan returns the canonical Claude Code install-skill FixPlan.
// Backward-compat alias for NewPlanFor("claude").
func NewPlan() acmm.FixPlan {
	plan, _ := NewPlanFor("claude")
	return plan
}

// Body is the canonical Claude Code skill body. Backward-compat for
// callers that imported it directly (tests, etc.).
const Body = claudeSkillBody

// ===== bodies =====
//
// The prose is shared (corePlumblineGuide). Each tool gets a thin
// wrapper that prepends frontmatter / preamble in that tool's
// expected format.

const corePlumblineGuide = `# plumbline workflow

Plumbline is a deterministic Go CLI that scans a repository for AI coding
readiness signals and reports its ACMM level (1–5). Detection is
deterministic — no LLM calls, no network. The CLI is the load-bearing
interface; an interactive Bubble Tea TUI is also available on terminals.

## When to invoke

- User asks "is this repo ready for AI coding?" or about AI maturity.
- User asks about adding CLAUDE.md / AGENTS.md / copilot-instructions /
  .cursorrules / .windsurfrules.
- User wants to improve PR templates, contributor guides, or commit rules.
- User asks "what's the next thing to add for better AI workflows?"
- A plumbline CI gate is failing and you need to know why.

## Recommended call sequence

1. **Get the verdict (machine-readable).**

   ` + "```" + `
   plumbline --json
   ` + "```" + `

   Parse ` + "`verdict.level`" + ` (1–5), ` + "`verdict.level_scores`" + ` (per-level
   averages), and ` + "`verdict.next_gap`" + ` (signals at L+1 not yet Found).

2. **For each signal in next_gap, get evidence + a fix recipe.**

   ` + "```" + `
   plumbline inspect <signal-id> --json
   ` + "```" + `

   Returns a ` + "`signal-result`" + ` object with ` + "`status`" + `, ` + "`score`" + `,
   ` + "`confidence`" + `, ` + "`evidence`" + ` (file paths + excerpts), ` + "`notes`" + `
   (the "why"), and ` + "`fix_hint`" + ` (the "how to fix").

3. **Preview a scaffolded fix; apply only with explicit confirmation.**

   ` + "```" + `
   plumbline fix <signal-id>           # dry-run, prints the plan
   plumbline fix <signal-id> --apply   # actually writes
   ` + "```" + `

   Inputs (project conventions, anti-patterns, etc.) can be supplied with
   repeatable ` + "`--input KEY=VALUE`" + ` pairs. Fixers refuse to overwrite
   existing files; if the target already exists they append a marked
   block instead.

4. **Discover the catalog.**

   ` + "```" + `
   plumbline signals --json
   plumbline schema verdict
   plumbline schema signal-result
   ` + "```" + `

## Stable contracts

- **Signal IDs** are stable across patch releases:
  - L2: ` + "`l2.agent-instructions`" + `, ` + "`l2.contributor-guide`" + `, ` + "`l2.pr-template`" + `, ` + "`l2.commit-rules`" + `
  - L3: build-lint-gate, coverage-gate, coverage-suite, nightly-compliance, flaky-analysis, error-monitoring, user-feedback, acceptance-tracking
  - L4: self-modifying-config, auto-triage, threshold-block, worktree-agents, error-recovery
  - L5: issue-to-pr, self-improvement, docs-from-prs, multi-repo-orchestration
- **JSON Schemas** (draft 2020-12) for verdict / signal-result / event / config
  via ` + "`plumbline schema <name>`" + `.
- **Exit codes**: 0 ok, 1 gate-failed (` + "`--fail-below`" + `), 2 cannot-run, 3 config-error.
- **Read-only by default.** Only ` + "`plumbline fix --apply`" + ` and
  ` + "`plumbline install-skill --apply`" + ` write inside the target repo.

## Notes for agents

- Plumbline collapses CLAUDE.md / AGENTS.md / copilot-instructions /
  .cursorrules / .windsurfrules into ONE signal (` + "`l2.agent-instructions`" + `).
  Whichever the team uses is fine; don't suggest adding all of them.
- The four-step partial-credit rubric is fixed: 0.0 / 0.33 / 0.67 / 1.0.
  Don't propose a "0.5" — it's not in the rubric.
- ` + "`--min-confidence high`" + ` is the right CI-gate strictness if the
  verdict can't tolerate low-confidence (filename-only) matches.
- Workflow signals (L3+) parse GitHub Actions YAML only in MVP; other
  CI systems are deferred behind ` + "`--ci-system`" + `.

## When NOT to invoke

- General code review (use the user's preferred review tool).
- Test coverage analysis itself (plumbline detects whether a coverage
  *gate* exists; it doesn't compute coverage).
- Static analysis on application code (use golangci-lint, eslint, etc.).
- Anything that requires running the user's tests / building their app.

## Reference

- Spec: ` + "`SPEC.md`" + ` in this repo (or in the plumbline repo if you've
  installed it via ` + "`go install`" + `).
- Source paper: https://arxiv.org/abs/2604.09388v1 (Andy Anderson,
  "The AI Codebase Maturity Model", IBM Research).
- Topical help: ` + "`plumbline help <topic>`" + ` (levels, signals, scoring,
  output, config, ci, agents, profiles, compatibility, fix).
`

// claudeSkillBody is the Claude Code skill (SKILL.md with frontmatter
// so Claude Code auto-discovers it under .claude/skills/<name>/).
const claudeSkillBody = `---
name: plumbline
description: Use plumbline to assess this repo's AI Codebase Maturity Model (ACMM) level and apply scaffolded fixes. Invoke when the user asks about AI coding readiness, missing instruction files (CLAUDE.md, AGENTS.md, copilot-instructions, .cursorrules, .windsurfrules), CI quality gates, contributor guides, PR templates, commit conventions, or "what should we add next to make this repo better for AI-driven development?"
---

` + corePlumblineGuide

// cursorRuleBody is a Cursor MDC rule file.
const cursorRuleBody = `---
description: How to use plumbline (an ACMM-based AI coding readiness assessor) when working in this repo.
globs:
  - "**/*"
alwaysApply: false
---

` + corePlumblineGuide

// geminiBody is for Gemini Code Assist (GEMINI.md / ~/.gemini/GEMINI.md).
const geminiBody = `<!-- GEMINI.md is consumed by Gemini Code Assist (CLI + IDE
extensions). plumbline wrote this guide here. -->

` + corePlumblineGuide

// agentsBody is for AGENTS.md (Codex CLI, OpenCode, etc.). Plain
// markdown with a tool-agnostic preamble — this file might be read
// by several agent tools at once.
const agentsBody = `<!-- AGENTS.md is consumed by OpenAI Codex CLI, OpenCode, and other
tools following the AGENTS.md convention. plumbline wrote this guide
here because the user selected one of those tools as the install
target. -->

` + corePlumblineGuide

// windsurfBody is for Windsurf / Cline single-file rules.
const windsurfBody = `<!-- Single-file agent rules consumed by Windsurf or Cline. plumbline
wrote this guide here. -->

` + corePlumblineGuide

// copilotBody is for .github/copilot-instructions.md.
const copilotBody = `<!-- GitHub Copilot project-level instructions. plumbline wrote this
guide here. -->

` + corePlumblineGuide
