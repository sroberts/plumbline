# Signal Catalog

_Auto-generated from `signals.Default` by `cmd/gen-signal-docs`. Do not edit by hand — the regeneration workflow at `.github/workflows/docs-signals.yml` opens a PR when this file drifts from the source._

Each signal is a deterministic detector that returns a status (`found` / `partial` / `missing` / `na`) plus a confidence and method. The full Result schema is at `plumbline schema signal-result`.

## L2 — Instructed

| ID | Family | Title |
|---|---|---|
| `l2.agent-instructions` | instructions | Agent instructions present (CLAUDE.md / AGENTS.md / copilot-instructions / etc.) |
| `l2.commit-rules` | templates | Commit-message conventions encoded in repo |
| `l2.contributor-guide` | instructions | Contributor / development guide present and substantive |
| `l2.pr-template` | templates | PR template with structured checklist |

## L3 — Measured

| ID | Family | Title |
|---|---|---|
| `l3.acceptance-tracking` | monitoring | Tracked metrics file (acceptance rates / auto-qa-tuning) |
| `l3.build-lint-gate` | ci-gate | CI workflow runs build and lint on push or PR |
| `l3.coverage-gate` | coverage | Coverage gate fails CI below a threshold |
| `l3.coverage-suite` | coverage | Scheduled (cron) coverage suite |
| `l3.error-monitoring` | monitoring | Error-monitoring SDK declared in dependency manifest |
| `l3.flaky-analysis` | compliance | Flaky-test tracking workflow or report file |
| `l3.nightly-compliance` | compliance | Scheduled compliance / a11y / perf / security suite |
| `l3.user-feedback` | feedback | User-feedback / NPS / survey channel wired up |

## L4 — Adaptive

| ID | Family | Title |
|---|---|---|
| `l4.auto-triage` | automation | Scheduled workflow that triages issues automatically |
| `l4.error-recovery` | automation | Workflow retries failures with backoff or continue-on-error |
| `l4.self-modifying-config` | automation | Workflow writes back to the repo (PR or push) |
| `l4.threshold-block` | automation | Workflow conditional reads metrics and gates on a threshold |
| `l4.worktree-agents` | automation | Concurrent AI agent runner / devcontainer / worktree config |

## L5 — Self-Sustaining

| ID | Family | Title |
|---|---|---|
| `l5.docs-from-prs` | automation | Documentation updates triggered by PR events |
| `l5.issue-to-pr` | automation | Issues open PRs automatically |
| `l5.multi-repo-orchestration` | automation | Workflow fans out across multiple repositories |
| `l5.self-improvement` | automation | Workflow updates instruction files based on merged PRs |

_21 signals total._
