# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

Plumbline is in a pre-implementation / seed state. The repo currently contains only:

- `README.md` — one-line project description.
- `the_ai_codebase_maturity_model.md` — the spec source (Andy Anderson's ACMM paper, Markdown converted via Microsoft MarkItDown).
- `2604.09388v1.pdf` — the original paper.

There is **no Go module, Makefile, source tree, CI config, or test suite yet.** Do not invent commands (`go test ...`, `make ...`) until those files actually exist — bootstrap them when needed and update this file in the same change.

## What Plumbline is

A standalone Go CLI that performs a **repo-level AI coding readiness assessment** based on the **AI Codebase Maturity Model (ACMM)** from `the_ai_codebase_maturity_model.md`. Given a target repository path, it should detect which feedback-loop artifacts are present and report the codebase's ACMM level.

The Markdown paper is the **specification**. When designing detection logic, scoring, or report output, treat that document as authoritative and reference its tables (especially the *Complete Feedback Loop Inventory*) rather than reasoning from the README alone.

## ACMM in one screen (what the tool must detect)

The model assigns a level by **feedback loop topology**, not by AI autonomy. Levels are sequential — Level N requires Level N-1's artifacts.

| Level | Name | Loop topology | Artifact families to detect |
|------|------|--------------|-----------------------------|
| 1 | Assisted | Open loop | Code only; no encoded context |
| 2 | Instructed | Human → AI | one of `CLAUDE.md` / `AGENTS.md` / `.github/copilot-instructions.md` / `.cursorrules` / `.windsurfrules`, plus contributor guide, PR template checklist, commit rules |
| 3 | Measured | AI → metrics → human | Coverage gates, nightly suites (compliance/perf/a11y/security), flaky-test analysis, error monitoring, NPS, acceptance-rate logs (e.g. `auto-qa-tuning.json`) |
| 4 | Adaptive | Loop closes without human in the path | Self-modifying configs, automated triage workflows, threshold-driven blocks, worktree-based concurrent agents |
| 5 | Self-Sustaining | Codebase *is* the policy | Issue-to-PR automation, self-improvement cycles that update guidance from merged-PR analysis |

The author's KubeStellar Console reference points (paths and cron cadences) are listed in Table 2 of the paper around lines 1042–1260 of `the_ai_codebase_maturity_model.md`. Use those as canonical examples of what L2/L3/L4 artifacts look like on disk.

**Plumbline does not follow Table 2 verbatim.** SPEC.md §6 has a "Deviations from the source paper" subsection documenting where the implemented signal catalog diverges and why. Most notably: L2 collapses CLAUDE.md / copilot-instructions / AGENTS.md into a single `l2.agent-instructions` signal — most teams use one agent, and requiring directives for several is too strict. Read SPEC.md before changing the catalog.

Two anti-patterns the assessor should also flag:

- **Dashboard graveyard** (L3 anti-pattern): metrics collected, never acted on.
- **Autonomy without guardrails** (L4 anti-pattern): automation present, but the L3 measurement layer it depends on is missing — a level cannot be skipped.

## Working norms specific to this repo

- The paper's `.md` is a MarkItDown conversion of a PDF. Expect odd line wrapping, hyphenation across lines, and stray whitespace. Don't "tidy" it — it's the source artifact, and downstream parsing should be robust to it. Quote line ranges rather than reflowing.
- The source paper is real: [arxiv.org/abs/2604.09388v1](https://arxiv.org/abs/2604.09388v1) ("The AI Codebase Maturity Model" by Andy Anderson, IBM Research). The local `the_ai_codebase_maturity_model.md` is the markdown conversion (via Microsoft MarkItDown) of the paper's PDF. Cite the arXiv URL when referencing claims; quote line ranges from the local `.md` when working with specific passages, since the conversion has whitespace/hyphenation quirks worth preserving.
