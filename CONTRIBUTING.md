# Contributing

Welcome. This guide is the canonical source for how PRs get reviewed
and merged in this repository — both for human contributors and for
AI coding agents.

## Workflow

1. Branch from main: `git checkout -b <type>/<short-name>`.
2. Make focused commits — one logical change per PR.
3. Run the full test suite locally before opening the PR.
4. Open a PR; the template lists the per-merge checks.

## What gets a PR rejected

- Tests don't cover the change (table-driven for branchy code; teatest
  for TUI flows; subprocess E2E for CLI binary behavior).
- Signal IDs renamed without a deprecation alias — they're part of the
  public contract per [SPEC.md §7](SPEC.md).
- New behavior added without updating SPEC.md or `plumbline help` topics.
- The skill body in `internal/skill/skill.go` diverged from what
  `plumbline install-skill` actually writes.
- LLM calls or network IO in the default path. Plumbline is
  deterministic by design — see SPEC.md §3.
- Signal scoring not on the four-step rubric (`0.0 / 0.33 / 0.67 / 1.0`).
- Direct `os.ReadFile` (or any direct IO) inside a Signal's `Detect`.
  Use `idx.Read(path)` — that's the access-contract chokepoint per
  SPEC.md §5.
- `gofmt`, `go vet`, or `go test -race ./...` not green locally before
  opening the PR.

## Local checks

Before opening a PR, run:

```bash
make build     # cross-check the ldflags
make test      # full unit + E2E suite
make test-race # race detector
make lint      # gofmt + go vet (golangci-lint if installed)
```

CI (`.github/workflows/ci.yml`) runs the same gate on every PR.

## Where things live

- **Public API:** `pkg/acmm` — types consumed by `--json` output.
- **Scanner / index:** `internal/scanner` — the IO chokepoint.
- **Signals:** `internal/signals/lN/<id>.go` — one file per detector.
  Adding a signal is a single-file change; register via `init()`.
- **Fix application:** `internal/fix` — the only place plumbline
  writes inside a target repo. Refuses overwrite, in-repo only.
- **Workflows AST:** `internal/workflows` — GitHub Actions YAML parser.
- **Embedded skill bodies:** `internal/skill` — what
  `plumbline install-skill` writes for each agent target.
- **TUI:** `internal/tui` — Bubble Tea screens + picker flows.

## Style

- Format with `gofmt`. Don't reformat unrelated lines; keep diffs minimal.
- Comments explain WHY, not WHAT — well-named identifiers carry the
  WHAT.
- Errors propagate up; don't swallow them silently.

## Asking for help

Open a draft PR or an issue. Don't sit on a stuck branch — small
feedback loops beat one heroic perfect PR.
