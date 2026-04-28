// Package skill holds the embedded Claude Code skill that
// `plumbline install-skill` writes into a target repo. Path and Body
// are exported so both the install-skill command and the TUI can
// reference them without depending on the cmd/plumbline package.
package skill

import "github.com/sroberts/plumbline/pkg/acmm"

// Path is the canonical location for project-local Claude Code skills.
// Each skill lives in its own directory under .claude/skills/.
const Path = ".claude/skills/plumbline/SKILL.md"

// Body is the SKILL.md content. Hand-tuned for AI agents: frontmatter
// (name + description) so the skill is auto-discoverable; concise body
// covering when to invoke, recommended call sequence, stable contracts,
// and when NOT to invoke.
const Body = `---
name: plumbline
description: Use plumbline to assess this repo's AI Codebase Maturity Model (ACMM) level and apply scaffolded fixes. Invoke when the user asks about AI coding readiness, missing instruction files (CLAUDE.md, AGENTS.md, copilot-instructions, .cursorrules, .windsurfrules), CI quality gates, contributor guides, PR templates, commit conventions, or "what should we add next to make this repo better for AI-driven development?"
---

# plumbline workflow

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

// NewPlan returns the canonical install-skill FixPlan that both the
// install-skill command and the TUI consume. Wrapping the constants
// in a function keeps the call sites uniform if per-repo customization
// is ever added.
func NewPlan() acmm.FixPlan {
	return acmm.FixPlan{
		SignalID: "install-skill",
		Summary:  "Install plumbline Claude Code skill at " + Path,
		Ops: []acmm.FixOp{{
			Kind: acmm.FixCreateFile,
			Path: Path,
			Body: []byte(Body),
		}},
	}
}
