# Plumbline — CLI Spec

A Go CLI that assesses a repository's AI Codebase Maturity Model (ACMM) level by detecting feedback-loop artifacts on disk. Interactive TUI via [Bubble Tea](https://github.com/charmbracelet/bubbletea); deterministic, offline by default.

Spec source of truth: `the_ai_codebase_maturity_model.md` (Andy Anderson's ACMM paper).

---

## 1. Goals

1. Given a repository path, classify its **ACMM level (1–5)** based on which feedback loops are wired up.
2. Produce a **per-signal evidence trail** — file paths, line ranges, regex matches — so a human can verify or contest the verdict.
3. Ship **two interfaces at full feature parity**:
   - **Interactive TUI** (Bubble Tea) — default when stdout is a TTY. Optimized for humans exploring a repo.
   - **Pure CLI** — flag-driven, no curses, no cursor manipulation, machine-friendly output. Optimized for **LLM tool callers**, CI gates, and shell scripts. Every TUI capability (scan, browse, drill-down, export, help) has a flag-driven equivalent. There is no feature reachable only through the TUI.
4. Be **fully self-contained**: no network, no API keys, no LLM calls in the default path.
5. Treat `--help` and `plumbline help <topic>` as a **first-class API surface**. An LLM agent should be able to drive the tool correctly using only its own help output — no external docs required.

## 2. Non-goals

- Grading the *prose quality* of `CLAUDE.md` or other instruction files.
- Linting application code or running the target repo's tests.
- Recommending specific tooling (Jest vs Vitest, etc.). The tool reports loop coverage; it doesn't pick stacks.
- Continuous monitoring / daemon mode. Plumbline is a one-shot scanner.

## 3. Deterministic vs LLM-assisted: decision

**Decision: deterministic-only for MVP. No LLM calls.** Build a `--enrich` extension hook so an optional LLM pass can be added later without changing the core data model.

**Rationale**

- The paper's central claim is that maturity is a function of **feedback-loop topology** — *which loops exist* and *how they're wired*, not how eloquently the docs read. Topology is observable from the filesystem.
- Reproducibility: the same commit must always score identically. LLM-as-judge introduces variance that breaks CI gates.
- Operational simplicity: no API keys, no rate limits, no network egress, no cost-per-run. Works offline and in air-gapped CI.
- Speed: a deterministic scan of a large repo finishes in under a few seconds; an LLM pass is orders of magnitude slower.
- Auditability: every "yes" comes with a file path and matched bytes. Reviewers can re-run grep and verify.

**What an optional LLM pass would unlock later (out of MVP scope)**

- Judging whether `CLAUDE.md` actually encodes *useful* conventions vs being a stub.
- Detecting custom feedback loops the regex/AST rules don't recognize.
- Summarizing scan results in natural language.

These are nice-to-haves layered on top of a deterministic core. They are gated behind `--enrich` and require an explicit `ANTHROPIC_API_KEY` (or equivalent). MVP does not implement them.

## 4. Command surface

```
plumbline [global flags] <command> [args]
```

### Commands

| Command | Purpose | TUI? |
|---|---|---|
| `plumbline assess [path]` | Scan a repo and report its ACMM level. Defaults to `.`. | yes (default) |
| `plumbline snapshot [path]` | Scan a repo and write a committable maturity-state artifact — `.plumbline.toon` by default (or `json`/`yaml` via `--format`). Sugar over `assess --report` with a repo-root default output path. | no |
| `plumbline inspect <signal-id> [path]` | Run a scan and print the **detail view** for a single signal — evidence, status, fix recipe. CLI equivalent of opening the TUI's detail screen. | no |
| `plumbline signals` | List every signal the tool knows how to detect. Filterable by `--level`, `--family`. Supports `--json`. | no |
| `plumbline explain <signal-id>` | Print a signal's description, detection rule, and rationale. **Static** — does not scan a repo. | no |
| `plumbline schema <name>` | Emit JSON Schema for a public output type (`verdict`, `signal-result`, `event`, `config`). For machine consumers. | no |
| `plumbline help [topic]` | Long-form help on a topic (`signals`, `scoring`, `levels`, `output`, `config`, `ci`, `agents`, `profiles`). Output is plain markdown so an LLM agent can ingest it directly. | no |
| `plumbline version` | Print build version + commit. Supports `--json`. | no |

### Global flags

| Flag | Default | Purpose |
|---|---|---|
| `--cli` | auto | Force pure-CLI mode (no Bubble Tea, no ANSI cursor manipulation). Auto-set when stdout is not a TTY. Implied by `--json`, `--report`, `--events`, `--quiet`. |
| `--tui` | auto | Force TUI even when stdout is not a TTY (rare; useful inside `script(1)` or for debugging). Mutually exclusive with `--cli`. |
| `--json` | off | Print results as JSON to stdout — a shortcut for `--report json`. Implies `--cli`. Output schema: `plumbline schema verdict`. |
| `--report <fmt>` | `toon` | Pick the report format. `fmt` ∈ `toon`, `json`, `yaml`, `markdown`, `sarif`. TOON is also the default CLI output when `--report`/`--json` are omitted. Implies `--cli`. |
| `--out <path>` | `-` | Report destination. `-` = stdout. |
| `--events <fmt>` | off | Stream scan progress events to **stderr**. `fmt` ∈ `ndjson`, `text`. Implies `--cli`. Schema: `plumbline schema event`. |
| `--quiet` / `-q` | off | Suppress banner, progress, and trailing hints. Errors still go to stderr. Implies `--cli`. |
| `--no-color` | auto | Disable ANSI color in CLI output. Auto-set when stdout is not a TTY or `NO_COLOR` is set. |
| `--config <path>` | `.plumbline.yml` if present | Override scoring thresholds and signal toggles. |
| `--fail-below <level>` | none | Exit non-zero if assessed level is below `<level>`. For CI gating. |
| `--profile <name>` | `default` | Named signal preset (`default`, `go-only`, `frontend-only`, `oss-cncf`). |
| `--include-signal <id>` | all | Only run the listed signals. Repeatable. Mutually exclusive with `--exclude-signal`. |
| `--exclude-signal <id>` | none | Skip the listed signals. Repeatable. |
| `--level <N>` | all | Only run signals at ACMM level `N` (2–5). Repeatable. |
| `--family <name>` | all | Only run signals in family `<name>`. Repeatable. |
| `--min-confidence <lvl>` | `low` | Only credit signals whose `confidence` is at or above `<lvl>` (`low`, `medium`, `high`). For CI gates that don't want to count filename-only matches. See §6 confidence ladder. |
| `--signal-set <ver>` | `latest` | Pin the signal rule-set version (e.g., `v1`). Stops the verdict from drifting when plumbline upgrades. See `plumbline help compatibility`. |
| `--ci-system <name>` | `auto` | CI flavor to detect. MVP supports `auto` and `github-actions`. Other systems (`gitlab-ci`, `buildkite`, `circleci`, `jenkins`) are M4+. See §6 CI-system scope. |
| `--debug` | off | Emit detection diagnostics on stderr: which paths each signal inspected, which patterns ran, what matched. Implies `--cli`. Does not change stdout — gate output stays stable. |
| `--clock <mode>` | `wall` | Timestamp source for events. `wall` (real time), `fixed` (deterministic, for golden-file tests; also via `PLUMBLINE_FIXED_TS`), `relative` (events start at `t=0`). |

### Mode selection

| Condition | Mode chosen |
|---|---|
| stdout is a TTY, no override flag | TUI |
| stdout is not a TTY (pipe / redirect / CI) | CLI |
| `--cli` set | CLI |
| `--tui` set | TUI |
| `--json`, `--report`, `--events`, or `--quiet` set | CLI (implied) |
| `assess` command invoked, but **any subcommand other than `assess`** | CLI (TUI is `assess`-only) |

The two modes share the **exact same scanner, signal registry, and scoring engine.** Only the output formatter and event sink differ.

### Exit codes

| Code | Meaning |
|---|---|
| 0 | Scan completed, level ≥ `--fail-below` (or no gate set). |
| 1 | Scan completed, level < `--fail-below`. |
| 2 | Scan could not run (path not a repo, IO error, etc.). |
| 3 | Configuration error. |

## 5. Architecture

```
cmd/plumbline/           # main package, cobra/urfave-cli wiring
internal/
  scanner/               # filesystem walker; produces a RepoIndex
  signals/               # one file per signal, registered into a Registry
    l2/                  # CLAUDE.md, copilot-instructions, PR template, ...
    l3/                  # coverage gate, nightly compliance, flaky-test job, ...
    l4/                  # self-modifying configs, auto-triage workflow, ...
    l5/                  # issue→PR pipeline, self-improvement cycles, ...
  scoring/               # rolls signal results into level verdict
  report/                # markdown / json / sarif emitters
  tui/                   # Bubble Tea models (scan, results, detail, help)
  config/                # .plumbline.yml loader
pkg/acmm/                # public types: Level, Signal, Evidence, Verdict
```

**Layering rule:** `signals` depends on `scanner`. `scoring` depends on `signals`. `tui` and `report` depend on `scoring`. Nothing in `internal/` imports `tui`. `pkg/acmm` is the only public surface.

### `RepoIndex`

The scanner walks the repo once, builds an index, and hands it to every signal. The index carries **metadata only** — no file content. This makes memory usage bounded and IO costs explicit.

```go
type RepoIndex struct {
    Root      string
    Files     []FileEntry         // path, size, mode (no content)
    ByName    map[string][]string // basename -> paths (fast lookup)
    Workflows []WorkflowFile      // parsed CI workflow ASTs (see CI-system scope, §6)
    GitMeta   GitMeta             // remote, default branch, HEAD commit (read-only)
    HasGit    bool

    // Read returns up to the first 64 KiB of a tracked file.
    // Files larger than 1 MiB are sampled (first 64 KiB).
    // Reads are cached for the duration of a scan.
    Read(path string) ([]byte, error)
}
```

**Access contract for signals (enforced by lint, not just convention):**

1. Signals read metadata directly from `RepoIndex` fields.
2. Signals that need file *content* MUST go through `idx.Read(path)`. This is the **only** sanctioned IO chokepoint and the place where the size cap, encoding handling, read-budget telemetry, and `--debug` diagnostic capture all live.
3. Signals MUST NOT call `os.Open`, `os.ReadFile`, `filepath.Walk`, or any other direct IO. An `internal/signals/...` lint rule fails CI on direct stdlib IO imports.
4. Signals MUST NOT spawn subprocesses or make network calls. Same lint.
5. Workflow YAML/HCL/Jenkinsfile parsing happens once in the scanner, producing a CI-system-agnostic `WorkflowFile` AST. Signals consume the AST, not raw bytes, so workflow-aware signals work uniformly across CI systems as we add them.

This keeps the tool hermetic, makes the IO budget testable, and ensures the §11 sandbox guarantees actually hold rather than relying on signal authors to remember them.

## 6. The signal model

Every detector is a `Signal`. Signals are the unit of pluggability and the unit a user can toggle in `.plumbline.yml`.

```go
type Signal interface {
    ID() string                           // e.g. "l2.claude-md"
    Level() acmm.Level                    // 2
    Family() string                       // "instructions"
    Title() string                        // "CLAUDE.md present and non-trivial"
    Detect(ctx context.Context, idx *RepoIndex) Result
}

type Result struct {
    Status     Status            // Found | Partial | Missing | NA
    Score      float64           // one of {0.0, 0.33, 0.67, 1.0} — see rubric below
    Confidence Confidence        // High | Medium | Low — see ladder below
    Method     Method            // FilenameMatch | ContentRegex | AST | CrossFile
    Evidence   []Evidence        // file paths, line ranges, captured strings
    Notes      []string          // human-readable explanations
    Diag       []DiagEntry       // populated only when --debug; see §8.2.6
}

type Evidence struct {
    Path    string             // repo-relative
    Span    *LineSpan          // optional
    Excerpt string             // optional, ≤200 chars
}

type DiagEntry struct {
    Path   string              // path inspected (repo-relative)
    Action string              // "stat" | "read" | "regex" | "ast-query"
    Hit    bool                // did this probe match?
    Detail string              // pattern, AST query, or note
}
```

### Partial-credit rubric (mandatory)

Every signal's `Score` comes from a fixed four-step ladder. Signal authors do not invent intermediate values. This is the difference between a coherent verdict and one that drifts as new signals land.

| Score | Status | Meaning | Example |
|---|---|---|---|
| `0.0` | `Missing` | Artifact absent. | No `CLAUDE.md`. |
| `0.33` | `Partial` | Named or stubbed only. | `CLAUDE.md` exists but `< 30` non-blank lines. |
| `0.67` | `Partial` | Present but incomplete. | Coverage workflow runs but no fail-on-threshold flag. |
| `1.0` | `Found` | Fully wired. | Coverage workflow + threshold + gating PRs. |

`Status` is derived from `Score`: `0.0 → Missing`, `1.0 → Found`, `{0.33, 0.67} → Partial`. A signal that cannot map to one of these four steps must be split into two signals.

### Confidence ladder

`Confidence` is **independent of `Score`** and answers a different question: how much should you trust the verdict?

| Confidence | Meaning |
|---|---|
| `High` | Verified by AST analysis or cross-file logic — hard to fool. |
| `Medium` | Content regex matched a substantive pattern (e.g., `--cov-fail-under` flag with a numeric threshold). |
| `Low` | Filename-only match, or content match against a generic word list. May produce false positives. |

CI gates use `--min-confidence high` for strictness: signals scoring `< 1.0` at lower confidence are then treated as `Missing` for gate purposes, but still surface in the report as informational. This gives the gate operator one knob — strictness — instead of forcing them to disable individual signals.

### CI-system scope (MVP: GitHub Actions only)

The L3/L4/L5 catalog as written **detects GitHub Actions only**. Workflow signals look at `.github/workflows/*.yml`. This is stated explicitly so:

- signal authors don't write GitHub-shaped rules and call them generic;
- users on other CI systems see a clear "not yet supported" instead of false negatives;
- the design pressure is on the scanner (which produces the `WorkflowFile` AST) to absorb new CI systems, not on every signal to re-implement them.

Other CI systems are deferred to **M4+** behind `--ci-system <name>`:

| System | File pattern | Status |
|---|---|---|
| GitHub Actions | `.github/workflows/*.yml` | MVP |
| GitLab CI | `.gitlab-ci.yml`, `.gitlab/*.yml` | M4+ |
| Buildkite | `.buildkite/pipeline.yml` | M4+ |
| CircleCI | `.circleci/config.yml` | M4+ |
| Jenkins | `Jenkinsfile`, `jenkins/*.groovy` | M4+ |

When `--ci-system` is set to a system the scanner cannot parse, workflow signals return `NA` (not `Missing`) and the verdict notes the CI system was unrecognized.

### Initial signal catalog (MVP)

Signals are derived directly from Table 2 ("Complete Feedback Loop Inventory") of the paper. Each is implemented as a deterministic check.

**Level 2 — Instructed**

| ID | Detection rule |
|---|---|
| `l2.agent-instructions` | At least **one** recognized agent-directive file is present at root, with a heading and ≥ 20 non-blank lines. Recognized files (priority order): `CLAUDE.md`, `AGENTS.md`, `.github/copilot-instructions.md`, `.cursorrules`, `.windsurfrules`. **See "Deviations from the source paper" below.** |
| `l2.contributor-guide` | `CONTRIBUTING.md` or `.github/CARD_DEVELOPMENT_GUIDE.md` (or analogue) present. |
| `l2.pr-template` | `.github/pull_request_template.md` (or `PULL_REQUEST_TEMPLATE/`) contains ≥ 3 markdown checkboxes (`- [ ]`). |
| `l2.commit-rules` | `.gitmessage`, `commitlint.config.*`, or `.github/commit-convention.md` present. |

**Level 3 — Measured**

| ID | Detection rule |
|---|---|
| `l3.build-lint-gate` | A workflow runs on `push`/`pull_request` and contains a build *and* lint step (string match against common tools or a `lint`/`build` job name). |
| `l3.coverage-gate` | A workflow on `pull_request` invokes a coverage tool with a threshold flag (`--cov-fail-under`, `coverage threshold`, `--minimum-coverage`, etc.) **or** a `codecov.yml`/`.codecov.yaml` defines a target. |
| `l3.coverage-suite` | Coverage workflow exists on a periodic schedule (`schedule: cron`) — captures the "full coverage suite" pattern. |
| `l3.nightly-compliance` | At least one scheduled workflow whose name/file matches `nightly`, `compliance`, `a11y`, `accessibility`, `security`, or `perf`. |
| `l3.flaky-analysis` | Scheduled workflow named `flaky*` or `*flaky*`, **or** a tracked file like `flaky-tests.json`. |
| `l3.error-monitoring` | Repo references an error monitor (Sentry SDK init, GA4 error tracking, OpenTelemetry exporter) in dependency manifests or source. |
| `l3.user-feedback` | NPS/CSAT survey component (filename match `*nps*`, `*survey*`) **or** a `feedback/` issue template. |
| `l3.acceptance-tracking` | A tracked file matching `auto-qa-tuning.json`, `acceptance-rates.*`, or analogous JSON in a `metrics/` directory. |

**Level 4 — Adaptive**

| ID | Detection rule |
|---|---|
| `l4.self-modifying-config` | A workflow uses `peter-evans/create-pull-request`, `git commit` + `git push`, or writes back to a tracked config file (e.g. `auto-qa-tuning.json`). |
| `l4.auto-triage` | Scheduled workflow that runs more than once per day (`cron` with hour wildcard) and uses GitHub Issues API or `gh issue` commands. |
| `l4.threshold-block` | A workflow conditional reads from a metrics file and fails based on a threshold (`if: fromJson(...).rate < N`). |
| `l4.worktree-agents` | Repo contains `.devcontainer/`, agent runner config (`.claude/`, `.github/agents/`), or scripts referencing concurrent worktrees. |
| `l4.error-recovery` | Workflows use `continue-on-error` + retry steps, or `nick-fields/retry` action. |

**Level 5 — Self-Sustaining**

| ID | Detection rule |
|---|---|
| `l5.issue-to-pr` | Workflow trigger includes `issues: [opened, labeled]` and the workflow opens a PR (uses `create-pull-request` or pushes a branch). |
| `l5.self-improvement` | Workflow that consumes merged-PR data (`pull_request: closed`) and writes back to instruction files (`CLAUDE.md`, guidance docs). |
| `l5.docs-from-prs` | Workflow that updates documentation when PRs merge (commit step targets `docs/`, `README.md`, or `web/docs/`). |
| `l5.multi-repo-orchestration` | `.github/workflows/*` references multiple repos via `gh api repos/...`, or contains a matrix over repo names. |

This catalog is **not** the final word. Each signal lives in its own file under `internal/signals/lN/<id>.go` and is registered through `init()`. Adding a signal is a one-file change.

### Deviations from the source paper

Plumbline mostly follows Table 2 of the ACMM paper, but a few signals have been intentionally restructured. We document those here so the divergence is visible to anyone reading both side-by-side.

#### L2: agent-instructions is one signal, not many

The paper (Table 2, lines 1054–1067) lists `CLAUDE.md` and `Copilot instructions` as **separate feedback loops**, both required at L2. The reference deployment (KubeStellar Console) ran Claude Code *and* GitHub Copilot in parallel, so requiring directives for both made sense in that context.

Plumbline collapses these into a single `l2.agent-instructions` that fires on the presence of **any one** of: `CLAUDE.md`, `AGENTS.md`, `.github/copilot-instructions.md`, `.cursorrules`, `.windsurfrules`.

**Why:**
- The vast majority of teams use one coding agent, not several. A repo with a substantive `CLAUDE.md` and no Copilot users was being penalized for not also encoding directives for a tool nobody on the team uses.
- The L2 *intent* is "encoded preferences exist." That is satisfied by any one of these files; whether it is named `CLAUDE.md` or `AGENTS.md` is a tooling choice, not a maturity signal.
- AGENTS.md, .cursorrules, and .windsurfrules postdate the paper and are now common in the wild. Pinning the rule to the two files the paper named would mean penalizing repos that adopted the newer conventions.

**Caveat:** teams that *do* genuinely use multiple agents in concert (the KubeStellar pattern) get the same single-signal credit as a single-tool team. We accept this loss of resolution at L2 because the catalog is about *encoding judgement*, not auditing tool coverage. Teams that want stricter enforcement can add custom signals via the future plugin mechanism (§13).

#### L3: PR-template checklist is L2, not L3

The paper places "PR template checklist" under L2 ("Instructed") in its description (lines 549–551) but its Table 2 entry hints at L2 / L3 ambiguity ("Every PR" frequency). Plumbline keeps it at L2: a checklist is encoded judgement, which is the L2 definition.

#### L3+: GitHub-Actions only in MVP

The paper describes 63 CI/CD workflows on GitHub Actions specifically. The L3+ workflow signals here parse `.github/workflows/*.yml` only. Other CI systems (GitLab CI, Buildkite, Jenkins) are deferred to M4+ behind `--ci-system` (see §6 CI-system scope). This is a scope cut, not a philosophical disagreement with the paper.

## 7. Scoring

### Per-signal

`Status` ∈ `{Found, Partial, Missing, NA}`. `Score` ∈ `[0, 1]`. `NA` excludes the signal from level math (e.g. a Go-only repo gets `NA` for a JS-specific signal).

### Per-level

```
levelScore(L) = sum(score for s in signals[L] if status != NA)
              / count(s for s in signals[L] if status != NA)
```

### Overall verdict

The repo is at level `L` iff:

1. `levelScore(k) ≥ T_pass` for every `k ∈ {2, ..., L}` (no skipping — the paper is explicit).
2. `levelScore(L+1) < T_pass` **or** `L == 5`.

`T_pass` defaults to `0.7`. Configurable via `.plumbline.yml`:

```yaml
thresholds:
  pass: 0.7
signals:
  l3.user-feedback: { enabled: false }   # opt out of an irrelevant signal
profile: go-only
```

The verdict object also reports the **next-level gap**: which specific signals at `L+1` are missing, ordered by impact. This is the actionable part — the user wants to know *what to add next*.

### Signal-set versioning

A repo's verdict must not silently flip just because plumbline upgraded. The signal *catalog* and the *detection rules behind each signal* are versioned together as a **signal set** (`v1`, `v2`, …).

**Compatibility policy within a major:**

- New signals can be added (so a previously-passing repo can pick up additional credit, never lose it).
- Existing rules can **only loosen** (never tighten) within a major.
- Tightening a rule, removing a signal, or renaming an ID requires a **major bump** and ships with `compat:` aliases for one minor cycle.

**Verdict shape:**

Every verdict JSON includes:

```json
"tool_version": "1.4.2",
"signal_set_version": "v1"
```

CI gates pin with `--signal-set v1`. If the pinned version is not available (retired, or the binary is too old/new), exit code 3 fires with a clear migration message. `plumbline help compatibility` lists what each version contains and what changed between them.

## 8. Interface modes

Plumbline ships **two interfaces at full feature parity**: the Bubble Tea TUI for humans and a pure CLI for LLM agents, CI, and scripts. Mode is auto-detected from the environment but always overridable via `--cli` / `--tui` (see §4 Mode selection).

There is **no functionality available only in TUI mode.** Every TUI screen and keybinding has a flag-driven equivalent listed in §8.2.1 below.

## 8.1 TUI mode (Bubble Tea)

The TUI uses Bubble Tea + Bubbles + Lip Gloss. Three top-level screens, navigated with `tab` / `shift-tab`. `?` shows help, `q` / `ctrl-c` quits.

### 8.1.1 Scan screen

Shown while signals run. Each signal runs in a goroutine; results stream back via a `tea.Msg` channel.

```
plumbline · scanning /path/to/repo
─────────────────────────────────────────────────
[████████████░░░░░░░░] 14/22 signals

✓ l2.claude-md            CLAUDE.md present
✓ l2.copilot-instructions Copilot instructions present
✓ l2.pr-template          PR template w/ 6 checkboxes
✗ l3.coverage-gate        no coverage threshold found
… l3.nightly-compliance   scanning .github/workflows/
```

### 8.1.2 Results screen

Default view after scan completes.

```
plumbline · /path/to/repo · level 3 (Measured)
─────────────────────────────────────────────────
 L2 Instructed   ████████████████████  100%  (5/5)
 L3 Measured     ██████████████░░░░░░   72%  (6/8)
 L4 Adaptive     ████░░░░░░░░░░░░░░░░   20%  (1/5)  ← next
 L5 Self-Sustain ░░░░░░░░░░░░░░░░░░░░    0%  (0/4)

To reach Level 4, add:
  • l4.self-modifying-config — no workflow writes back to repo
  • l4.auto-triage           — no sub-daily scheduled workflow
  • l4.threshold-block       — no metric-driven workflow conditionals

[enter] drill in   [e] export report   [s] signals list   [?] help
```

The level bars and "next gap" panel are the load-bearing UI — that is what the user came for.

### 8.1.3 Detail screen

Selecting a signal opens its evidence:

```
l3.coverage-gate · MISSING
─────────────────────────────────────────────────
A workflow on pull_request must invoke a coverage
tool with a fail-on-threshold flag.

Looked at:
  .github/workflows/ci.yml         (no coverage step)
  .github/workflows/test.yml       (coverage runs, no threshold)
  codecov.yml                      (not present)

Fix: add a `--cov-fail-under=80` flag to the
test step in test.yml, or commit a codecov.yml
with a target.

[esc] back   [o] open in $EDITOR
```

## 8.2 CLI mode

CLI mode is the **non-interactive, machine-friendly** interface. It is what runs in CI and what an LLM agent invokes through a tool-use harness. It produces three streams:

- **stdout** — the result, in the format requested by `--json` or `--report`, or the default TOON encoding when neither is set.
- **stderr** — progress events (when `--events` is set), warnings, errors, and trailing hints.
- **exit code** — verdict gate result (see §4).

CLI mode never repositions the cursor, never clears the screen, never reads from stdin, and never blocks waiting for input. Output is line-oriented and safe to compose with `tee`, `xargs`, `jq`, log forwarders, and LLM tool-use frameworks.

### 8.2.1 Feature parity with the TUI

| TUI screen / action | CLI equivalent |
|---|---|
| Scan progress (streaming dots and spinner) | `plumbline assess --events ndjson` (events stream to stderr; final result to stdout) |
| Results overview (level bars + next-level gap) | `plumbline assess` — default TOON output, or `--json` for JSON |
| Detail screen for a single signal | `plumbline inspect <signal-id>` |
| `e` — export markdown report | `plumbline assess --report markdown --out report.md` |
| `s` — switch to signals list | `plumbline signals --level 3 --family compliance` |
| `?` — help overlay | `plumbline help <topic>` and `<command> --help` |
| Filter / search within results | `--include-signal`, `--exclude-signal`, `--level`, `--family` |
| `o` — open file in `$EDITOR` | (omitted; CLI emits absolute paths so the caller can open them itself) |

### 8.2.2 Default output: TOON (`plumbline assess`)

In CLI mode `plumbline assess` emits **TOON** (Token-Oriented Object Notation) by default — the same compact, diff-friendly, lossless encoding of the full `Report` produced by `--report toon` (see §9). It is written to stdout unless `--out` redirects it. JSON and YAML are the alternatives, selected with `--json` (a shortcut for `--report json`) or `--report yaml`; `markdown` and `sarif` emit summaries rather than the full data.

```
schema: plumbline/v1
repo: /path/to/repo
verdict:
  level: 3
  name: Measured
  next_gap[3]: l4.auto-triage,l4.self-modifying-config,l4.threshold-block
signals[19]{id,level,status,score,confidence}:
  l2.agent-instructions,2,found,1,high
  ...
```

The level bars, per-level pass badges, and `Hint:` lines from earlier releases are the **TUI** results screen (see §8.2.1); on a terminal you get that interactive view, while pipes and CI get TOON. A consumer that wants the verdict level reads `verdict.level`; the actionable next steps are `verdict.next_gap`.

### 8.2.3 Detail output (`plumbline inspect <signal-id>`)

Same content as the TUI detail screen, plain-text, no curses:

```
$ plumbline inspect l3.coverage-gate
l3.coverage-gate · MISSING

A workflow on pull_request must invoke a coverage tool with a
fail-on-threshold flag, or a codecov.yml must define a target.

Looked at:
  /abs/.github/workflows/ci.yml          (no coverage step)
  /abs/.github/workflows/test.yml:42     (coverage runs, no threshold)
  /abs/codecov.yml                       (not present)

Fix:
  Add `--cov-fail-under=80` to the test step in test.yml,
  or commit a codecov.yml with target.coverage.

See also:
  plumbline explain l3.coverage-gate
  plumbline help scoring
```

`--json` flips this to a single `signal-result` object; schema at `plumbline schema signal-result`.

### 8.2.4 NDJSON event stream

`--events ndjson` emits one JSON object per line to **stderr**, while stdout still receives the final result. This lets a caller separate progress logging from the actual answer:

```
plumbline assess --json --events ndjson 2>events.log >verdict.json
```

Event types (schema published via `plumbline schema event`):

```json
{"event":"scan.start","ts":"2026-04-28T15:00:00.001Z","repo":"/abs/path","signal_count":22}
{"event":"signal.start","ts":"2026-04-28T15:00:00.012Z","id":"l2.claude-md"}
{"event":"signal.complete","ts":"2026-04-28T15:00:00.015Z","id":"l2.claude-md","status":"found","score":1.0,"duration_ms":3}
{"event":"scan.complete","ts":"2026-04-28T15:00:00.342Z","level":3,"duration_ms":342}
```

Events are monotonic in `ts`. `--events text` emits the same data in a human-readable single-line form (for tailing in CI logs).

### 8.2.5 CLI ergonomics for LLM agents

These are concrete affordances, not aspirations. Each is testable.

- **Stable IDs.** Signal IDs (`l3.coverage-gate`) are part of the public contract. Renames go through a deprecation cycle with a `compat:` alias for at least one minor version.
- **Stable schemas.** Anything that comes out of `--json` or `--events ndjson` is described by a JSON Schema at `plumbline schema <name>`. Schema `$id` includes `plumbline/v1/...`.
- **Stable exit codes.** §4's exit-code table is part of the contract; values do not change in patch releases.
- **No stdin reads.** CLI commands never block on stdin. Anything that would prompt in TUI mode requires a flag in CLI mode.
- **Idempotent.** Running `assess` twice on the same commit (with the same flags and config) produces identical stdout, identical exit code, and identical event sequences modulo timestamps and durations.
- **Hint chains.** Errors and partial results print a `Hint:` line on stderr pointing at the next valid command (`plumbline help <topic>` or `plumbline <command> --help`).
- **Discoverability via `--json`.** `signals --json`, `explain --json`, `help --json` and `version --json` all emit structured output, so an agent never has to scrape human prose.

### 8.2.6 Color palette

CLI mode uses a fixed, contractual color palette. Tooling that scrapes terminal output can rely on it; humans get a consistent visual language.

| Token | ANSI | Meaning |
|---|---|---|
| Red | `31` | `missing` — signal absent. |
| Yellow | `33` | `partial` — present but incomplete. |
| Green | `32` | `found` — fully wired. |
| Dim | `2` | `na` — not applicable to this repo (different stack, opted out). |
| Cyan | `36` | Hint lines and `See also:` cross-references. |
| Bold | `1` | Headings, level names, signal IDs. |

Color is suppressed when **any** of the following hold: stdout is not a TTY, `--no-color` is set, `NO_COLOR` env var is set (any value). Color is forced when `CLICOLOR_FORCE=1` is set, even on non-TTYs. Markdown reports use checkbox/badge equivalents (`✓` / `~` / `✗`) instead of ANSI; SARIF severity follows the same mapping (`note` / `warning` / `error`).

### 8.2.7 `--debug` diagnostics

When the user disagrees with a `Missing` verdict, they need to see what the detector actually looked at. `--debug` is the answer.

`--debug` emits one structured line per probe to **stderr**. Stdout is unchanged — gate output remains stable. Output format is human-readable text by default, NDJSON when `--events ndjson` is also set.

```
$ plumbline assess --debug
…
[debug] l3.coverage-gate: stat .github/workflows/ci.yml         hit=true
[debug] l3.coverage-gate: regex `--cov-fail-under` in ci.yml    hit=false
[debug] l3.coverage-gate: stat .github/workflows/test.yml       hit=true
[debug] l3.coverage-gate: regex `--cov-fail-under` in test.yml  hit=false
[debug] l3.coverage-gate: stat codecov.yml                      hit=false
[debug] l3.coverage-gate: result = missing (score=0.0, conf=high, method=content-regex)
…
```

Each `DiagEntry` (see §6 `Result` struct) becomes one line. The `--debug` flag is the load-bearing affordance for trust: a user who sees exactly which paths were probed and which patterns ran can either fix their repo or file a precise issue against a signal.

`--debug` is also captured into the verdict JSON when both `--debug` and `--json` are set: each `signal-result` gains a `diag: [...]` array. This lets LLM agents reason about *why* a signal fired the way it did without re-running the scan.

## 9. Output formats

### `--json`

```json
{
  "schema": "plumbline/v1",
  "tool_version": "1.4.2",
  "signal_set_version": "v1",
  "ci_system": "github-actions",
  "repo": "/abs/path",
  "scanned_at": "2026-04-28T15:00:00Z",
  "verdict": {
    "level": 3,
    "name": "Measured",
    "level_scores": {"2": 1.0, "3": 0.72, "4": 0.20, "5": 0.0},
    "next_gap": ["l4.self-modifying-config", "l4.auto-triage", "l4.threshold-block"],
    "min_confidence_applied": "low"
  },
  "signals": [
    {
      "id": "l2.claude-md",
      "level": 2,
      "family": "instructions",
      "status": "found",
      "score": 1.0,
      "confidence": "high",
      "method": "content-regex",
      "evidence": [{"path": "CLAUDE.md", "excerpt": "# CLAUDE.md\\n\\nThis file..."}],
      "notes": []
    }
  ]
}
```

The `min_confidence_applied` field reflects the `--min-confidence` value used when computing the gate. Each signal entry gains an optional `diag: [...]` array when `--debug` was set (see §8.2.7).

### `--report markdown`

Mirrors the TUI results screen but as a static document, suitable for committing to `docs/maturity.md` or pasting into a PR. Includes the verdict, per-level bars (rendered as ASCII or shields-style badges), and a "next gap" checklist.

### `--report sarif`

Each `Missing` signal becomes a SARIF result with severity `note` and `ruleId == signal.ID()`. This lets GitHub's code-scanning tab surface plumbline findings inline on PRs.

### `--report toon` / `--report yaml`

Lossless re-encodings of the same `Report` structure emitted by `--report json`. All three are produced from one generic tree (the report is marshaled to JSON, then re-decoded), so field names, `omitempty` elisions, and values are identical across the three notations — only the surface syntax differs.

- **TOON** ([Token-Oriented Object Notation](https://github.com/toon-format/spec)) is the compact, LLM-token-efficient default for the `snapshot` artifact: uniform arrays of objects collapse to a CSV-like table under a single field header, primitive arrays render inline, and every array declares its length so a consumer can verify completeness.
- **YAML** is offered as a "force" format for tooling that prefers it.

Map keys are emitted in sorted order in both, keeping the artifact deterministic and diff-friendly.

### `plumbline snapshot`

`snapshot` runs the standard assess pipeline and writes the full report as a **committable maturity-state artifact**. It is the low-friction path to the file teams commit and track over time:

- Default format **TOON**, default output **`<repo>/.plumbline.toon`** (a repo-root dotfile, not a file in the caller's CWD).
- `--format toon|json|yaml` selects the notation and the default extension (`.plumbline.json`, `.plumbline.yaml`).
- `--out <path>` overrides the destination; `--out -` streams to stdout and writes no file.
- Signals disabled in `.plumbline.yml` are honored, exactly as in `assess`.

`snapshot` is intentionally a thin wrapper over `assess --report`; the behavioral differences are the repo-root default output path and reproducible-by-default output (below). Use `assess --report toon|yaml|json` when you want the same encodings with assess's full flag surface (filters, profiles, events) and the live, un-normalized report.

#### Reproducibility & the CI drift gate

A committed artifact is only useful if it diffs cleanly — a file that churns on every scan is noise, not signal. So `snapshot` is **reproducible by default**: the two fields that vary by *when and where* the scan ran, rather than by the codebase's maturity, are normalized to stable values:

- `scanned_at` → a fixed RFC3339 sentinel (`1970-01-01T00:00:00Z`). Still schema-valid (the field is required and typed `date-time`), but constant.
- `repo` → the repository directory's base name, not the absolute path (stable across `/home/user/...` locally vs `/home/runner/work/...` in CI).

`tool_version` and `signal_set_version` are left intact — they are intrinsic to *how* signals were scored, so a change in either is a real change worth surfacing in the diff. Re-running `snapshot` on an unchanged repo therefore produces a byte-identical file. Pass `--reproducible=false` to embed the live scan time and absolute path (for a per-run artifact you upload rather than commit).

This makes a **CI drift gate** a one-liner: regenerate the artifact and fail if it moved.

```yaml
- run: |
    go build -o /tmp/plumbline ./cmd/plumbline
    /tmp/plumbline snapshot --out .plumbline.toon .
    git diff --exit-code -- .plumbline.toon \
      || { echo "::error::.plumbline.toon is stale — run 'plumbline snapshot' and commit"; exit 1; }
```

Every change that moves plumbline's assessment then shows up as a reviewable diff in the PR instead of silently rotting. This is the L3 measurement loop applied to the repo itself — the artifact is collected **and acted on**, avoiding the *dashboard graveyard* anti-pattern (metrics gathered but never gating anything). plumbline runs exactly this gate on itself in `.github/workflows/ci.yml`.

## 9.5 Help & discoverability

Help output is a **first-class deliverable**, not boilerplate. The success criterion: an LLM agent that has read only `plumbline --help` and `plumbline help agents` should be able to drive every feature correctly without external documentation.

### `--help` contract for every command

Each command's `--help` output **must** include:

1. A one-line synopsis (`plumbline assess — scan a repo and report its ACMM maturity level`).
2. A 2–3 sentence description: what the command does, when to use it, and how it differs from sibling commands.
3. Every flag with: type, default, env-var override (if any), one-line meaning, and "see also" cross-references.
4. **At least three examples** — one TTY example, one machine-pipe example, and one CI/agent example.
5. Exit-code semantics if the command can fail meaningfully.
6. A `See also:` footer listing related commands and `plumbline help` topics.

#### Example: `plumbline assess --help`

```
plumbline assess — scan a repo and report its ACMM maturity level

Synopsis:
  plumbline assess [path] [flags]

Description:
  Walks the repository at [path] (default ".") and runs every enabled
  signal detector against it. Produces a verdict (level 1–5), per-level
  scores, and the list of signals that would unlock the next level.

  Mode is auto-detected: TUI on a terminal, CLI when piped or in CI.
  Use --cli to force non-interactive output, --tui to force the TUI.

Flags:
  --json                 Emit JSON to stdout (shortcut for --report json).
                         Implies --cli. Schema: `plumbline schema verdict`.
  --report fmt           Report format. fmt: toon|json|yaml|markdown|sarif.
                         Default CLI output is toon. Implies --cli.
  --out path             Report destination. "-" = stdout. Default "-".
  --events fmt           Stream progress events to stderr.
                         fmt: ndjson|text. Schema: `plumbline schema event`.
  --include-signal id    Only run the listed signals (repeatable).
                         IDs from `plumbline signals --json`.
  --exclude-signal id    Skip the listed signals (repeatable).
  --level N              Only run signals at level N (2–5). Repeatable.
  --family name          Only run signals in family <name>. Repeatable.
                         Families from `plumbline signals --json`.
  --fail-below N         Exit 1 if assessed level < N. For CI gates.
  --profile name         Named signal preset. See `plumbline help profiles`.
  --config path          Override config path. Default: .plumbline.yml.
  --cli, --tui           Force interface mode (see --help for details).
  --quiet, -q            Suppress banners, progress, and trailing hints.
  --no-color             Disable ANSI color (also via NO_COLOR=1).

Examples:
  # Interactive TUI: scan the current directory.
  plumbline assess

  # Machine-readable, with progress events on stderr.
  plumbline assess --json --events ndjson 2>events.log >verdict.json

  # CI gate: fail if not at level 3 or higher.
  plumbline assess --fail-below 3 --quiet

  # Scan a specific repo, save markdown report.
  plumbline assess /path/to/repo --report markdown --out maturity.md

  # Only run the L3 coverage signals.
  plumbline assess --level 3 --family coverage --json

Exit codes:
  0  scan completed; gate passed (or no gate set)
  1  scan completed; gate failed (assessed level < --fail-below)
  2  could not run (path not a directory, IO error)
  3  configuration error

See also:
  plumbline inspect          drill into one signal's evidence
  plumbline signals          list every signal the tool detects
  plumbline help scoring     how levels and the no-skip rule are computed
  plumbline help agents      guidance for LLM tool callers
```

`inspect`, `signals`, `explain`, `schema`, `help`, and `version` follow the same template.

### Topical help (`plumbline help <topic>`)

Long-form prose help for cross-cutting topics. Output is plain markdown — no ANSI, no boxes — so an LLM agent can ingest it directly. Each topic has a stable URL fragment (`plumbline help scoring#no-skip-rule`) that error messages can deep-link to.

| Topic | Content |
|---|---|
| `plumbline help` (no topic) | Index of all topics; the most common workflows; one-paragraph orientation. |
| `plumbline help levels` | The five ACMM levels, summarized from the paper, with citation pointers. |
| `plumbline help signals` | What a signal is, its lifecycle, status values (`found`/`partial`/`missing`/`na`), partial-credit semantics. |
| `plumbline help scoring` | Threshold math, the no-skip rule, how `next_gap` is computed, how `--fail-below` interacts. |
| `plumbline help output` | Every output mode, with example fragments and links to schemas. |
| `plumbline help config` | The `.plumbline.yml` schema, every key, every default. |
| `plumbline help ci` | How to wire plumbline into CI; copy-pasteable GitHub Actions and GitLab CI YAML. |
| `plumbline help agents` | **For LLM tool callers.** Recommended call sequences, error-handling patterns, schema-fetching workflow, gotchas. |
| `plumbline help profiles` | Named signal presets and exactly which signals each enables/disables. |

#### `plumbline help agents` — outline

This topic exists specifically so an LLM agent can self-orient:

1. *Start here:* call `plumbline schema verdict` and `plumbline schema event` to load the output contracts.
2. *Discover signals:* `plumbline signals --json` returns the full registry.
3. *Run an assessment:* `plumbline assess --json --events ndjson 2>events.log >verdict.json`.
4. *Drill into a missing signal:* for each entry in `verdict.next_gap`, call `plumbline inspect <id> --json`.
5. *Static reasoning without scanning:* `plumbline explain <id> --json`.
6. *Error handling:* exit codes and the `Hint:` lines; how to recover from `--config` errors.
7. *Stability guarantees:* signal IDs, schema `$id`, exit codes; what changes between versions.

### `plumbline schema <name>`

Emits a JSON Schema (draft 2020-12) for a public output type. Available names:

- `verdict` — top-level result of `assess --json`.
- `signal-result` — one signal's entry within a verdict (also the shape of `inspect --json`).
- `event` — NDJSON event line emitted by `--events ndjson`.
- `config` — `.plumbline.yml` schema.

Schemas are versioned: `$id` includes `plumbline/v1/...`. Backwards-incompatible changes bump the major version and ship a deprecation alias for one minor version.

### Discoverability principles

- Every error message ends with a `Hint:` line pointing at a relevant flag, command, or `plumbline help` topic.
- Unknown flag → list closest valid flags by Levenshtein distance.
- Unknown signal ID → list valid IDs at the same level/family.
- Help text is **stable across patch releases**. The wording of an exit-code description is part of the contract and tested as such (see §12).
- Every command supports `--json` for whatever output it normally produces in human form (`signals --json`, `explain --json`, `help --json`, `version --json`). An agent should never have to scrape prose.

## 10. Configuration

`.plumbline.yml` at repo root, optional. All keys optional.

```yaml
profile: default                 # default | go-only | frontend-only | oss-cncf

thresholds:
  pass: 0.7                      # min levelScore to count a level as "achieved"

signals:
  l3.user-feedback:
    enabled: false               # disable a signal entirely
  l3.coverage-gate:
    args:
      min_threshold: 75          # signal-specific tuning

paths:
  ignore:                        # additional gitignore-style patterns
    - vendor/
    - node_modules/
```

The config schema lives in `internal/config` and is JSON-Schema-validated on load. Unknown keys are a hard error (typos shouldn't silently disable signals).

## 11. Security & sandboxing

- **Read-only operation.** The tool never writes inside the target repo. Reports go to stdout or the path passed to `--out`.
- **Bounded file reads.** Any single file ≥ 1 MiB is sampled (first 64 KiB only). All content access goes through `idx.Read(path)` — see §5 access contract.
- **No subprocess execution.** Plumbline does not shell out for any reason. `git` metadata is read via go-git.
- **No telemetry. No crash reports. No usage data. Ever.** Plumbline makes zero outbound network calls. This is a positioning commitment, not a default that can be flipped — there is no flag or env var that turns network IO on. If you see plumbline opening a socket, file a security bug.
- **Excluded paths.** `.git/`, `node_modules/`, `vendor/`, and gitignored paths are skipped by default. Configurable via `paths.ignore` in `.plumbline.yml`.
- **Symlink handling.** Symlinks are followed only if the target resolves *inside* the repo root. Symlinks pointing outside the root are recorded as evidence (so a signal can mention them) but not traversed.
- **Submodules.** Not entered by default. `--include-submodules` opts in (M4+).
- **Text encoding.** Plumbline assumes UTF-8 file content. Files with a BOM or detected as UTF-16 / UTF-32 cause the affected signal to return `NA` with a `Notes` entry naming the encoding. Mis-detection is treated as user-fixable, not silently scored.
- **Platform support.** MVP targets darwin and linux on amd64 and arm64. Windows is M4+ — Bubble Tea is cross-platform, but path separators, CRLF, and TTY detection differ enough to warrant explicit testing rather than "should work."

## 12. Testing strategy

- **Signal tests**: each signal has a table-driven test with a set of in-memory `RepoIndex` fixtures (using `fstest.MapFS`-style synthetic trees). Both positive and negative cases.
- **Golden-file tests** for `--json`, `--report markdown`, default text output, and `inspect` output against a curated `testdata/` of mini-repos, one per ACMM level.
- **TUI snapshot tests**: `teatest` (Bubble Tea's testing harness) drives the model through scan → results → detail and snapshots terminal frames.
- **CLI snapshot tests**: every CLI command (`assess`, `inspect`, `signals`, `explain`, `schema`, `help`, `version`) has a golden-file diff under fixed seeds and a fixed `RepoIndex`. Catches accidental drift in machine-consumed output.
- **Help-text contract tests**: a single test iterates every command and asserts (a) `--help` output is non-empty, (b) every flag has a description, default, and example block, (c) exit-code documentation matches the runtime exit-code constants, (d) `See also:` cross-references resolve. Help drift fails CI.
- **Schema contract tests**: every emitted JSON Schema validates against draft-2020-12 itself; sample outputs validate against their own schema; changing a schema's structure without bumping `$id` fails CI.
- **Mode-selection tests**: matrix of (`stdin TTY?`, `stdout TTY?`, flag combos) → expected mode (TUI vs CLI). Asserts `--json`/`--report`/`--events`/`--quiet` correctly imply `--cli`.
- **Deterministic test mode**: `--clock fixed` (also via `PLUMBLINE_FIXED_TS=...`) pins event timestamps and durations so golden-file diffs on `--events ndjson` are byte-stable. Map iteration is sorted in any code path that emits to stdout/stderr. A dedicated test fails CI if a non-deterministic source (uncached `time.Now()`, unsorted `range` over a map at an emit boundary) sneaks into a `cmd/` or `report/` package.
- **Compatibility tests**: each signal-set version has frozen `testdata/` fixtures and frozen golden verdicts. A change to a signal that flips a fixture's verdict requires a major version bump in the signal set; the test enforces this rather than relying on review.
- **End-to-end**: `scripts/e2e.sh` runs `plumbline assess` against `testdata/` fixtures in both modes (CLI via `--cli`, TUI via `teatest`) and diffs the result.

## 13. Out-of-scope (deferred)

- **LLM enrichment** (`--enrich`) — see §3.
- **Watching mode / continuous scoring.** One-shot only.
- **A web dashboard.** Plumbline emits SARIF and JSON; visualization is someone else's job.
- **Suggesting specific tooling** (e.g., "use Vitest"). The tool reports topology; ecosystems make their own choices.
- **Non-GitHub CI systems.** See §6 CI-system scope. MVP is GitHub Actions only; GitLab CI, Buildkite, CircleCI, and Jenkins are M4+ behind `--ci-system <name>`.
- **Monorepo / per-workspace scoring.** Real users pointed at `kubernetes/kubernetes` or a Turborepo will want one verdict per workspace, not one for the whole tree. **Future shape (committed):** `plumbline assess --scope ./apps/web` runs the full assessor against a subtree and emits a single verdict for that scope; `--scope ./apps/*` fans out and emits one verdict per match. Until then, a single repo gets a single verdict.
- **External signal plugins.** Out of MVP. **Future shape (committed):** plugins are subprocesses invoked as `<plugin> --repo <path> --json`; they emit one or more `signal-result` JSON objects on stdout (schema: `plumbline schema signal-result`). Plugins are declared in `.plumbline.yml plugins:` with a path and a SHA-256 attestation. Explicitly **not** Go's `buildmode=plugin` — it's deprecated in practice and tied to exact toolchain versions.
- **Windows support.** See §11. Cross-platform is real work, not a `GOOS` flip.
- **Internationalized file content.** UTF-16/UTF-32 files return `NA` for affected signals — see §11.

## 14. Milestones

| M | Deliverable |
|---|---|
| M1 | **CLI mode end-to-end** against the L2/L3 signal catalog: `assess`, `inspect`, `signals`, `explain`, `schema`, `help`, `version`. `--json`, `--events ndjson`, `--fail-below`, full `--help` for every command, `plumbline help agents`. Help-text and schema contract tests in CI. No TUI yet. |
| M2 | Bubble Tea TUI: scan + results screens. Mode selection + `--cli`/`--tui` flags. TUI/CLI parity matrix tests. |
| M3 | TUI detail screen, markdown report, SARIF report, full `plumbline help` topic set. |
| M4 | L4 + L5 signal catalog complete; `.plumbline.yml` config; profile presets (`go-only`, `frontend-only`, `oss-cncf`). |
| M5 | Optional `--enrich` LLM hook (only if there is concrete demand for prose-quality grading). |

## 15. Open questions

1. Is "must not skip a level" a hard rule or a warning? Current spec: hard rule. The paper supports this, but a repo with `levelScore(2) = 0.6, levelScore(3) = 0.9` is interesting and the user might want it surfaced. Proposal: report it as `level: 2` with a `notes: ["L3 signals over-provisioned for L2 floor"]`.
2. Should `l5.self-improvement` require detecting that *instruction files have changed* over the last N days, or is the workflow's existence enough? MVP: existence is enough. A historical-trend mode is a separate feature.
3. Profile presets are useful but easy to over-engineer. MVP ships only `default`; others land when there's evidence they're needed.
