package main

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"
)

// helpTopics holds the prose for each `plumbline help <topic>` page.
// Output is plain markdown so an LLM agent can ingest directly.
var helpTopics = map[string]string{
	"levels":        helpLevels,
	"signals":       helpSignals,
	"scoring":       helpScoring,
	"output":        helpOutput,
	"config":        helpConfig,
	"ci":            helpCI,
	"agents":        helpAgents,
	"profiles":      helpProfiles,
	"compatibility": helpCompatibility,
	"fix":           helpFix,
}

func newHelpCmd(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "help [topic]",
		Short: "Long-form help on a topic, or the topic index when called bare",
		Long: `plumbline help — long-form, prose help for cross-cutting topics.

Output is plain markdown so an LLM agent can ingest it directly. Each
topic has a stable URL fragment (e.g. 'plumbline help scoring#no-skip-rule')
that error messages can deep-link to.

Without an argument, prints the topic index. With a topic, prints that
topic's full text.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				printTopicIndex(stdout)
				return nil
			}
			topic := args[0]
			body, ok := helpTopics[topic]
			if !ok {
				return errCannotRun(fmt.Errorf("unknown topic: %q. Run 'plumbline help' for the list", topic))
			}
			fmt.Fprintln(stdout, body)
			return nil
		},
	}
	return cmd
}

func printTopicIndex(w io.Writer) {
	keys := make([]string, 0, len(helpTopics))
	for k := range helpTopics {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Fprintln(w, "plumbline help — topical guides")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Topics:")
	for _, k := range keys {
		fmt.Fprintf(w, "  plumbline help %s\n", k)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Run 'plumbline <command> --help' for per-command flag reference.")
}

const helpLevels = `# ACMM Levels

The AI Codebase Maturity Model (ACMM) defines five levels by **feedback loop topology** — which loops exist and how they're wired — not by AI autonomy. Levels are sequential: Level N requires Level N−1's artifacts.

## Level 1 — Assisted (open loop)
The human initiates every interaction. AI is sophisticated autocomplete. No persistent context between sessions. The implicit floor — every repo starts here.

## Level 2 — Instructed (one-way: human → AI)
Preferences and conventions are encoded into instruction files (CLAUDE.md, .github/copilot-instructions.md, contributor guides, PR templates). AI output becomes consistent across sessions.

## Level 3 — Measured (AI → metrics → human)
Quantitative signals about agent / CI performance: coverage gates on every PR, nightly compliance suites, flaky-test analysis, error monitoring, NPS, acceptance-rate logs.

## Level 4 — Adaptive (loop closes without human in the path)
The system acts on its own metrics. Self-modifying configs, automated triage workflows, threshold-driven blocks, worktree-based concurrent agents, error recovery.

## Level 5 — Self-Sustaining (code is the policy)
Issues open PRs automatically. Self-improvement cycles update guidance from merged-PR analysis. Multi-repo orchestration. The community steers; AI implements.

See: 'plumbline help scoring' for how levels are computed.
`

const helpSignals = `# Signals

A **signal** is one detector. Each signal:
- Has a stable ID (e.g., 'l2.claude-md').
- Belongs to one ACMM level (2–5).
- Returns a Result with: status, score, confidence, method, evidence.

## The four-step rubric

Every signal's Score is one of four values — signal authors do not invent intermediates.

| Score | Status | Meaning                          |
|-------|--------|----------------------------------|
| 0.0   | missing| Artifact absent.                 |
| 0.33  | partial| Named or stubbed only.           |
| 0.67  | partial| Present but incomplete.          |
| 1.0   | found  | Fully wired.                     |

## Confidence

Confidence is **independent** of Score and answers a different question: how much should you trust the verdict?

- **high**: AST analysis or cross-file logic — hard to fool.
- **medium**: content regex matched a substantive pattern.
- **low**: filename-only match. May produce false positives.

CI gates use 'plumbline assess --min-confidence high' for strictness — see 'plumbline help scoring'.

## Method

How the signal arrived at its result:
- **filename** — name match only.
- **content-regex** — pattern matched in file content.
- **ast** — parsed-AST query (e.g., GitHub Actions workflow).
- **cross-file** — multiple files inspected together.

Run 'plumbline signals' for the full registry.
`

const helpScoring = `# Scoring

## Per-level math
levelScore(L) = avg(score) across non-NA signals at level L, after the min-confidence downgrade.

Empty levels (no signals registered) score 0 — we cannot pass a level we have no evidence for.

## No-skip rule
Verdict.Level is the **highest L** where every k in 2..L meets the pass threshold. The climb stops at the first level that fails. A repo with stellar L3 but missing L2 is L1.

## Pass threshold
Default: 0.7. Configurable via .plumbline.yml or '--threshold' (future flag).

## Min-confidence downgrade
Signals scoring < 1.0 at confidence below the gate are treated as 0.0 for verdict purposes. The maximum score (1.0) is honored regardless of confidence.

Use '--min-confidence high' to refuse credit for filename-only / weak matches in CI gates.

## next_gap
The next-level gap names signals at L+1 (the level above the verdict) that are not yet found, sorted alphabetically by ID. This is the actionable list — the user wants to know **what to add next**.
`

const helpOutput = `# Output Modes

plumbline emits results in several formats. Schemas at 'plumbline schema <name>'.

## Default human-readable text
'plumbline assess' — one-screen summary: level bars, next-gap panel, hint to use --json.

## JSON ('--json')
'plumbline assess --json' — full Report (schema: 'plumbline schema verdict'). Stable structure, machine-friendly.

## Markdown report ('--report markdown')
'plumbline assess --report markdown --out maturity.md' — committable summary.

## NDJSON event stream ('--events ndjson')
One JSON object per line emitted to stderr while the scan runs. Schema: 'plumbline schema event'.

  plumbline assess --json --events ndjson 2>events.log >verdict.json

## inspect / signals / explain / schema all support --json
Every command has structured output for LLM tool callers. See 'plumbline help agents'.
`

const helpConfig = `# Configuration (.plumbline.yml)

Optional file at the repo root. All keys optional. Schema: 'plumbline schema config'.

  profile: default              # default | go-only | frontend-only | oss-cncf

  thresholds:
    pass: 0.7

  signals:
    l3.user-feedback:
      enabled: false            # disable a signal entirely

  paths:
    ignore:
      - vendor/
      - node_modules/

Unknown keys are a hard error — typos shouldn't silently disable signals.
`

const helpCI = `# Wiring plumbline into CI

## GitHub Actions

  name: ACMM gate
  on: pull_request
  jobs:
    plumbline:
      runs-on: ubuntu-latest
      steps:
        - uses: actions/checkout@v4
        - uses: actions/setup-go@v5
          with:
            go-version: stable
        - run: go install github.com/sroberts/plumbline/cmd/plumbline@latest
        - run: plumbline assess --fail-below 3 --quiet

Exit codes:
  0  scan ok, gate passed (or no gate set)
  1  scan ok, gate failed (level < --fail-below)
  2  could not run (path / IO error)
  3  configuration error

Pin the signal set to refuse silent verdict drift on tool upgrade:
  plumbline assess --fail-below 3 --signal-set v1
`

const helpAgents = `# For LLM Tool Callers

plumbline is designed to be driven from an LLM tool harness. Recommended call sequence:

1. **Discover the contract.** Call 'plumbline schema verdict', 'plumbline schema event', 'plumbline schema signal-result'. Cache them locally.

2. **Discover signals.** Call 'plumbline signals --json'. Parse the array of signal descriptors.

3. **Run an assessment.** Use:
     plumbline assess --json --events ndjson 2>events.log >verdict.json
   Stdout is the final Report (schema: verdict). Stderr (events.log) is one JSON event per line.

4. **Drill into missing signals.** For each id in verdict.next_gap:
     plumbline inspect <id> --json
   The output is a single signal-result object.

5. **Handle errors.** Exit codes are part of the contract:
     0  ok
     1  gate failed
     2  could not run
     3  configuration error
   Stderr always carries an 'error: ...' line on non-zero exits, often followed by a 'Hint: ...' line pointing at the recovery command.

6. **Stability guarantees.**
   - Signal IDs are stable; renames go through a deprecation cycle.
   - JSON Schema $id is versioned (plumbline/v1/...). Major bumps for breaking changes.
   - Exit codes are part of the contract; values do not change in patch releases.
`

const helpProfiles = `# Signal Profiles

Named presets that enable / disable subsets of the signal catalog.

## default
All registered signals enabled. The default if no profile is named.

## go-only
Disables JS/TS-specific signals (ESLint, Prettier-only checks, etc.). Useful for Go-only repos so disabled signals don't drag the level score down.

## frontend-only
Disables backend-specific signals. For pure web frontends.

## oss-cncf
Adds extra strictness for CNCF-style open-source projects: stricter coverage gate, a11y nightly suite required for L3.

Note: profile presets in MVP ship 'default' only. Others land in M4+.
`

const helpFix = `# Applying Fixes

plumbline can scaffold or extend a repo's L2 instruction artifacts —
the files that turn an L1 "Assisted" repo into an L2 "Instructed" one.

This is the **only** path through which plumbline writes inside the
target repo (see SPEC.md §11). Everything else is read-only.

## Two ways to apply a fix

### CLI (one-shot, scriptable)

  # Dry-run: see what would be written.
  plumbline fix l2.claude-md

  # Actually write.
  plumbline fix l2.claude-md --apply

  # Provide inputs up front.
  plumbline fix l2.claude-md --apply \
      --input "project_summary=A Go CLI for X." \
      --input "conventions=- Use UV for Python envs.\n- No raw SQL."

### TUI (interactive)

In the detail screen for a fixable signal, press **a** (apply). The
TUI walks you through any input fields, shows a preview, and asks for
confirmation before writing.

Signals that have a fixer are marked with **✚** in the results screen.

## Safety guarantees

Every Apply call enforces:

- Paths in the FixPlan must be relative and stay inside repo root.
- ` + "`create-file`" + ` refuses to overwrite an existing file.
- ` + "`append-file`" + ` requires the target file to already exist.
- Dry-run is the default; ` + "`--apply`" + ` is required to write.
- Unknown FixOpKinds are rejected (no implicit broadening).

## What signals can fix

Currently the L2 catalog (file scaffolding):

- l2.claude-md            scaffolds CLAUDE.md or appends to existing
- l2.copilot-instructions scaffolds .github/copilot-instructions.md
- l2.contributor-guide    scaffolds CONTRIBUTING.md
- l2.pr-template          scaffolds .github/pull_request_template.md
- l2.commit-rules         scaffolds .gitmessage

L3+ fixes (workflow scaffolding) are deferred — they need more design
to handle the "merge into existing workflow" case safely.

## When to NOT use fix

- Custom existing instruction files: read what plumbline would
  generate (dry-run), then hand-edit instead. The fix is a starting
  point, not a final answer.
- Anything that would override prose you wrote: plumbline appends
  rather than overwriting, but the appended block is template-y.
`

const helpCompatibility = `# Compatibility & Signal-Set Versioning

Every Verdict carries:
  tool_version       e.g. 1.4.2 (semver)
  signal_set_version e.g. v1   (rule-set major)

## Within a major version of signal_set_version
- New signals can be added (a previously-passing repo can pick up additional credit, never lose it).
- Existing rules can **only loosen** (never tighten).

## Major bump triggers
- Tightening a detection rule.
- Removing or renaming a signal.

## Pinning in CI
Use '--signal-set v1' so the verdict cannot silently flip when plumbline upgrades:

  plumbline assess --fail-below 3 --signal-set v1

If the pinned version is unavailable (retired, tool too old/new), exit code 3 fires with a migration message.
`
