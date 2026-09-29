# plumbline

![ACMM level](.plumbline-badge.svg)

Repo-level AI coding readiness assessment, in Go.

Plumbline scans a repository, looks for the feedback loops that make AI-driven development reliable (instruction files, coverage gates, nightly suites, automated triage, issue-to-PR pipelines), and reports which level of the **AI Codebase Maturity Model (ACMM)** the repo sits at. Detection is deterministic — no LLM calls, no network.

Based on Andy Anderson's paper [*The AI Codebase Maturity Model: From Assisted Coding to Self-Sustaining Systems*](https://arxiv.org/abs/2604.09388v1) (the paper's `.md` is included as `the_ai_codebase_maturity_model.md`, converted from PDF via Microsoft MarkItDown). See **[Deviations from the source paper](#deviations-from-the-source-paper)** below for where this implementation deliberately diverges.

---

## Install

```bash
go install github.com/sroberts/plumbline/cmd/plumbline@latest
```

Pre-built binaries (darwin / linux × amd64 / arm64) ship on the [Releases](https://github.com/sroberts/plumbline/releases) page once a version is tagged.

## Quick start

```bash
# Bare invocation: opens the Bubble Tea TUI on a terminal.
plumbline

# Pipe / CI: full report as TOON (compact, diff-friendly).
plumbline | tee maturity.toon

# Machine-readable verdict for an LLM tool harness.
plumbline --json

# Stream progress events to stderr while the scan runs.
plumbline --json --events ndjson 2>events.log >verdict.json

# CI gate: fail if not at level 3.
plumbline --fail-below 3 --quiet

# Write a committable maturity-state artifact (.plumbline.toon by default).
plumbline snapshot
plumbline snapshot --format yaml   # or json

# Drill into one signal's status, evidence, and fix recipe.
plumbline inspect l2.agent-instructions

# Scaffold a missing artifact (dry-run; --apply to actually write).
plumbline fix l2.agent-instructions
plumbline fix l2.agent-instructions --apply

# Install a Claude Code skill so AI agents in your repo know how to drive plumbline.
plumbline install-skill --apply

# Scaffold .github/dependabot.yml from the manifests this repo actually has.
# (A utility — it moves no verdict. See "Dependency hygiene" below.)
plumbline install-dependabot --list     # what would it cover?
plumbline install-dependabot --apply

# Render the README status badge (self-contained SVG, no third-party service).
plumbline badge

# Scaffold the GitHub Actions workflow that runs plumbline (dry-run; --apply to write).
plumbline install-ci --fail-below 3
plumbline install-ci --fail-below 3 --apply
```

## ACMM levels

The model assigns a level by **feedback loop topology**, not by AI autonomy. Levels are sequential — Level *N* requires Level *N−1*'s artifacts.

| Level | Name | Loop topology | What plumbline looks for |
|------|------|---------------|--------------------------|
| 1 | Assisted | Open loop | (implicit floor — no checks needed) |
| 2 | Instructed | Human → AI | one agent-directive file (AGENTS.md / CLAUDE.md / copilot-instructions / .cursorrules / .windsurfrules / .cursor/rules / GEMINI.md), CONTRIBUTING.md, PR template, commit rules |
| 3 | Measured | AI → metrics → human | **verification harness** (one local command runs the tests; CI runs them too), opinionated lint/format config, machine-verifiable contracts (OpenAPI / Protobuf / GraphQL / JSON Schema), build/lint gate, coverage gate, scheduled compliance / a11y / perf / security, flaky-test analysis, error monitoring, NPS, acceptance tracking |
| 4 | Adaptive | Loop closes itself | self-modifying configs, sub-daily auto-triage, threshold-driven blocks, worktree agents, error recovery |
| 5 | Self-Sustaining | Codebase *is* the policy | issue-to-PR pipelines, self-improvement, docs-from-PRs, multi-repo orchestration |

A repo with stellar L3 but missing L2 is L1 — you cannot skip levels.

### Verification is the gate

An agent is bounded by how quickly and deterministically it can *check* its work, not by how fast it can generate it. Plumbline encodes that in three L3 signals and one rule:

- `l3.verification-harness` — tests exist, **one local command runs them** (`make test`, an npm `test` script, `go test ./...`), a lint/check command exists, and a push/PR workflow runs the tests. A basic test beats none: it is the anchor an agent iterates against.
- `l3.lint-config` — committed, *opinionated* lint and format configuration (golangci-lint, ruff, biome, eslint, prettier). A linter running on defaults is not the same as a repo that has declared its style.
- `l3.api-contract` — a machine-verifiable interface contract (OpenAPI, Protobuf, GraphQL, JSON Schema) that CI validates. Repos with no wire interface get `NA`, not a penalty.
- **The gate:** if `l3.verification-harness` scores below 0.67, the verdict is **capped at L2** however the rest of L3 scores, and the verdict names it under `capped_by`. Dashboards do not compensate for an agent that cannot run the tests. See [SPEC.md §7](SPEC.md#verification-gate).

At L2, `AGENTS.md` (the open multi-agent convention, including nested per-package files) counts alongside `CLAUDE.md`, Copilot instructions, `.cursor/rules/*.mdc`, and the other agent-rule files; structured PR templates remain the bridge from human intent to checkable criteria.

Run `plumbline signals` for the 26-signal catalog or `plumbline help levels` for the long form.

## Deviations from the source paper

Plumbline mostly follows Table 2 of the paper, but a few signals are intentionally restructured. **Full rationale is in [SPEC.md §6 → "Deviations from the source paper"](SPEC.md#deviations-from-the-source-paper).** Headline deltas:

- **L2 agent-instructions is ONE signal, not many.** The paper lists `CLAUDE.md` and `Copilot instructions` as separate L2 feedback loops (the reference deployment ran both in parallel). Plumbline collapses them, plus `AGENTS.md`, `.cursorrules`, and `.windsurfrules`, into a single `l2.agent-instructions` that fires on the presence of **any one**. Most teams use one agent; penalizing a project for not encoding directives for tools nobody uses was the wrong call.
- **PR-template lives at L2**, not L3 — the paper is ambiguous; "encoded judgment via checklist" is the L2 definition.
- **L3+ workflow signals are GitHub-Actions only in MVP.** GitLab CI / Buildkite / CircleCI / Jenkins are deferred behind `--ci-system`; not a philosophical disagreement, just a scope cut.

## Output formats

| Mode | Use case |
|---|---|
| TUI (default on a terminal) | Interactive exploration; drill into signals; apply fixes |
| TOON (default when piped / in CI) | Compact, diff-friendly full report to stdout |
| `--json` | Machine-readable verdict as JSON (shortcut for `--report json`); LLM tool harnesses |
| `--report yaml` | Full report as YAML |
| `--report markdown --out maturity.md` | Committable report |
| `--report sarif --out plumbline.sarif` | GitHub code-scanning |
| `snapshot` → `.plumbline.toon` | Committable maturity-state artifact (TOON; `--format json\|yaml` to force) |
| `badge` → `.plumbline-badge.svg` | Committable SVG status badge for the README |
| `--events ndjson` on stderr | Per-signal progress events while scanning |

TOON is the default CLI output: on a terminal you get the TUI, and everywhere else (pipes, CI) `plumbline assess` emits TOON. `--report json` (or `--json`), `toon`, and `yaml` are lossless re-encodings of the same report — same fields, different notation. [TOON](https://github.com/toon-format/spec) is compact and token-efficient, which is why it's both the default output and the default `snapshot` format.

Schemas are published via `plumbline schema {verdict, signal-result, event, config}` (draft 2020-12).

## TUI keybindings

| Key | Action |
|---|---|
| `↑` / `↓` (or `j` / `k`) | Move selection in the signal list |
| `enter` | Open the detail screen for the selected signal |
| `a` | Apply the signal's fix (only on signals marked `✚`) |
| `r` | Re-run the scan in place |
| `d` | Scaffold `.github/dependabot.yml` from the repo's manifests |
| `i` | Install plumbline's usage guide for a coding-agent tool (target picker) |
| `w` | Install the GitHub Actions workflow that runs plumbline (variant picker) |
| `esc` | Back |
| `q` / `ctrl-c` | Quit |

In the fix flow: `tab`/`shift+tab` between input fields, `enter` advances, `y`/`n` to confirm or cancel the preview.

## Apply fixes (safety)

`plumbline fix`, `plumbline install-skill`, `plumbline install-ci`, and `plumbline install-dependabot` are the only paths through which plumbline writes inside the target repo (`plumbline badge` also writes, but only its own SVG, at a path you name). Defaults are conservative:

- Dry-run by default; `--apply` is required to actually write.
- `create-file` refuses to overwrite an existing file.
- `append-file` requires the target to already exist.
- Paths must be relative and resolve inside the repo root; `..` and absolute paths are rejected.
- Unknown `FixOpKind`s are rejected.

Everything else (`assess`, `inspect`, `signals`, `explain`, `schema`, `help`, `version`) is read-only.

## Run plumbline in CI

Let plumbline write the workflow:

```bash
plumbline install-ci --fail-below 3          # dry run — prints what it would write
plumbline install-ci --fail-below 3 --apply  # write .github/workflows/plumbline.yml
```

Three variants (`--variant`, or press `w` in the TUI):

| Variant | What the workflow does |
|---|---|
| `full` (default) | Fail below a level **and** keep the README badge current |
| `gate` | Fail the build when the repo assesses below a minimum ACMM level |
| `badge` | Regenerate the README badge and fail if the committed copy is stale |

If you customize the badge with `plumbline badge --label`, pass the same value to `install-ci --badge-label`: the workflow regenerates the badge before diffing it, so a mismatch fails the gate on every run.

Omit `--fail-below` to install the measurement without the enforcement — worth doing first in a repo that isn't yet at the level it wants, since a gate that's red on the day it lands gets deleted rather than fixed.

**Scope note.** This is the only workflow plumbline will ever generate: the one that runs plumbline. It writes no coverage gate, no nightly suite, no triage automation — those stay advisory (`fix_hint`), for the reasons in [SPEC.md §4](SPEC.md). The generated workflow earns the repo **no** credit: `l3.build-lint-gate` deliberately ignores it, so plumbline never scores a repo for a file plumbline wrote. A test pins that.

### Status badge

`plumbline badge` renders a **self-contained SVG** you commit and reference with a relative path:

```bash
plumbline badge   # writes .plumbline-badge.svg
```

```markdown
![ACMM level](.plumbline-badge.svg)
```

Self-hosted, not a shields.io endpoint. plumbline makes no network calls, and a committed SVG keeps that true for the reader too: it renders in private repos, behind a proxy, and offline. The cost of self-hosting is staleness — so gate it. The SVG is byte-stable for an unchanged verdict, which makes regenerate-and-diff sufficient:

```yaml
- run: |
    plumbline badge --out .plumbline-badge.svg .
    git diff --exit-code -- .plumbline-badge.svg \
      || { echo "::error::badge is stale — run 'plumbline badge' and commit"; exit 1; }
```

A drifted badge then shows up as a reviewable change in the PR that caused it, rather than as a claim about the repo that quietly stopped being true. The badge at the top of this README is generated exactly this way and gated in [`.github/workflows/maturity.yml`](.github/workflows/maturity.yml).

### As a composite action

plumbline ships an [`action.yml`](action.yml), so a consumer repo can skip the install steps:

```yaml
- uses: sroberts/plumbline@v1
  with:
    fail-below: '3'
    badge: .plumbline-badge.svg
```

`v1` is a moving major tag: it tracks the newest 1.x release, so you pick
up fixes without editing your workflow — the same contract as
`actions/checkout@v4`.

Pin an exact release (`@v1.0.1`) instead when you need verdict-for-verdict
reproducibility. A plumbline upgrade can legitimately move a level, and an
exact pin means that only happens on a run you triggered.

Inputs: `path`, `version`, `go-version`, `fail-below`, `badge`, `badge-label`, `badge-drift-check`, `snapshot`, `summary`. Outputs: `level`, `level-name`, `next-gap`. The gate step runs **last**, so a failing gate still leaves the badge and job summary behind — which is the run where you most want them.

### By hand

```yaml
# .github/workflows/maturity.yml
name: ACMM gate
on: pull_request
jobs:
  plumbline:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: stable }
      - run: go install github.com/sroberts/plumbline/cmd/plumbline@latest
      - run: plumbline --fail-below 3 --quiet --signal-set v1
```

`--signal-set v1` pins the rule-set version so the gate can't silently flip when plumbline upgrades. `plumbline help compatibility` documents what each version contains.

### Dependency hygiene (outside the assessment)

`plumbline install-dependabot` writes `.github/dependabot.yml` from the manifests your repo actually has — one update block per manifest, plus `github-actions` whenever you have workflows:

```bash
plumbline install-dependabot --list    # what would it cover?
plumbline install-dependabot --apply
```

Pinned action versions are the case most repos miss. They rot like any dependency but have no manifest to hint at it, so a runtime deprecation takes every workflow out at once.

**This command sits outside the assessment loop, deliberately.** Every other remedy plumbline offers reaches you through `assess` → `next_gap` → `fix`/`fix_hint`. This one doesn't appear there, no signal detects the file, and running it moves no verdict.

Two reasons. A signal crediting the config would be plumbline scoring a file plumbline wrote — the circularity [SPEC.md §4](SPEC.md) exists to prevent. And the loop that *would* genuinely rate — updates merging without a human, gated on CI — is one this command doesn't write; it emits update blocks and a schedule, nothing that merges. Adding that signal was considered and declined; [SPEC.md §13](SPEC.md) records the shape it would have taken and why not.

So: dependency hygiene is worth having. It isn't a maturity level, and plumbline won't tell you it is.

### Track maturity over time (snapshot drift gate)

Commit a `.plumbline.toon` snapshot and let CI keep it honest. The snapshot is reproducible by default (volatile `scanned_at` / `repo` fields normalized), so it only diffs when the assessment actually changes — turning every maturity shift into a reviewable line in the PR.

```yaml
# In your CI job, after building plumbline:
- run: |
    plumbline snapshot --out .plumbline.toon .
    git diff --exit-code -- .plumbline.toon \
      || { echo "::error::.plumbline.toon is stale — run 'plumbline snapshot' and commit"; exit 1; }
```

Regenerate locally with `plumbline snapshot` and commit the result. plumbline runs this gate on itself in [`.github/workflows/ci.yml`](.github/workflows/ci.yml); the committed [`.plumbline.toon`](.plumbline.toon) is its own current maturity state.

### Comment the maturity delta on merge (`plumbline diff`)

Because the snapshot is re-readable, `plumbline diff` compares two artifacts and reports the level move plus which signals changed status:

```bash
git show origin/main:.plumbline.toon > /tmp/base.toon   # base — already committed
plumbline snapshot --out /tmp/head.toon .               # head — one scan
plumbline diff /tmp/base.toon /tmp/head.toon            # → PR-comment-ready delta
```

The base is read from the committed artifact, so a "how did this PR move maturity?" comment scans only the head — no second full assessment. plumbline does this on every merged PR in [`.github/workflows/verdict-delta.yml`](.github/workflows/verdict-delta.yml).

## Use plumbline from a coding agent

Install the plumbline-usage guide into the location your tool expects. Pass `--target <name>` to pick:

| Target | Path | Notes |
|---|---|---|
| `claude` (default) | `.claude/skills/plumbline/SKILL.md` | dedicated |
| `cursor` | `.cursor/rules/plumbline.mdc` | dedicated |
| `gemini` | `GEMINI.md` | shared |
| `codex` | `AGENTS.md` | shared (also read by other AGENTS.md tools) |
| `opencode` | `AGENTS.md` | shared |
| `windsurf` | `.windsurfrules` | shared |
| `cline` | `.clinerules` | shared |
| `copilot` | `.github/copilot-instructions.md` | shared |

```bash
# This repo (default = claude).
plumbline install-skill --apply

# Cursor in this repo.
plumbline install-skill --target cursor --apply

# Gemini, globally (under $HOME).
plumbline install-skill --target gemini --global --apply

# List all targets.
plumbline install-skill --list
```

`--global` installs at user scope (e.g., `~/.claude/skills/plumbline/SKILL.md`, `~/.gemini/GEMINI.md`) for tools that have a documented global location. Run `--list` for the full set; targets without one will error if you pass `--global`.

The TUI surfaces the same picker: bare `plumbline` on a terminal → press **`i`** → pick a target with `↑/↓` → toggle `[g]` for global ↔ project scope → enter to preview → `y` to install.

The body is hand-tuned for AI agents — when to invoke plumbline, the recommended call sequence, the stable signal IDs / schemas / exit codes, and when **not** to invoke it. The frontmatter is tailored per tool (Claude SKILL.md frontmatter, Cursor `.mdc` frontmatter, plain markdown for AGENTS.md / .windsurfrules / etc.).

## For LLM tool callers (without the skill)

Recommended call sequence:

1. `plumbline schema verdict` — fetch the output contract.
2. `plumbline signals --json` — discover the catalog.
3. `plumbline assess --json --events ndjson 2>events.log >verdict.json` — run the scan.
4. For each id in `verdict.next_gap`: `plumbline inspect <id> --json` to get evidence + fix recipe.
5. To scaffold: `plumbline fix <id> --apply --input KEY=VALUE …` (or `--json` for a structured plan).

Stable IDs, stable JSON-Schema `$id`s, stable exit codes (0 ok / 1 gate-failed / 2 cannot-run / 3 config-error). `plumbline help agents` has the full guidance.

## Documentation

- [SPEC.md](SPEC.md) — full spec: signal catalog, scoring math, schemas, deviations, milestones
- [CLAUDE.md](CLAUDE.md) — guidance for AI agents contributing to plumbline itself
- `the_ai_codebase_maturity_model.md` — the source paper
- `plumbline help` — topical guides (`levels`, `signals`, `scoring`, `output`, `config`, `ci`, `agents`, `profiles`, `compatibility`, `fix`)

## References & Prior Art

- **Source model.** Andy Anderson (IBM Research), [*The AI Codebase Maturity Model*](https://arxiv.org/abs/2604.09388v1) — defines the five levels by feedback-loop topology.
- **Talk.** Eno Reyes (Factory AI), ["Making Codebases Agent Ready"](https://www.youtube.com/watch?v=ShuJ_CN6zr4), AI Engineer. Its argument shaped the verification signals above: autonomous engineering agents are limited by *verification*, not by how well the model generates code — the **asymmetry of verification**, where checking a change is far cheaper to automate than producing one. The practical consequence is a shift from asking "how good is the model?" (code-generation benchmarks) to asking "how ready is the environment?" (can an agent verify its own work quickly, deterministically, and at scale?). Plumbline's `l3.verification-harness`, `l3.lint-config`, `l3.api-contract`, the `AGENTS.md` recognition at L2, and the verification gate are its attempt to make that readiness measurable from the filesystem. The talk motivates these signals; the detection rules and thresholds are plumbline's own ([SPEC.md §6](SPEC.md#verification-pillars-and-their-source)).

## Status

v0.1.0 — first release. Single-developer project; the catalog covers 26 signals across L2–L5. The fix scaffolder covers the L2 catalog; L3+ fix scaffolders are intentionally deferred (the "merge into existing workflow" case needs more design).

## License

MIT — see [LICENSE](LICENSE).
