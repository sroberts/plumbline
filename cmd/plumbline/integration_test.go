// Comprehensive cross-cutting integration tests. These exercise
// SPEC.md contract claims that span multiple subsystems and aren't
// covered by the per-package unit tests or the focused e2e tests in
// assess_test.go / main_test.go / contract_test.go / etc.
//
// Each test names the §-numbered claim it verifies so a failure
// points back at the spec line that promised the behavior.
package main

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// ===========================================================================
// SPEC §8.2.5 — Idempotency
// "Running assess twice on the same commit (with the same flags and config)
// produces identical stdout, identical exit code, and identical event
// sequences modulo timestamps and durations."
// ===========================================================================

// timestampedFields names the keys we have to scrub before comparing
// two verdict JSONs for equality. Anything else changing between runs
// is a regression — a re-scan of a frozen tree must be deterministic.
var timestampedFields = []string{"scanned_at"}

func scrubTimestamps(t *testing.T, raw string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, raw)
	}
	for _, k := range timestampedFields {
		delete(doc, k)
	}
	return doc
}

func TestIntegration_AssessIsIdempotent_SameStdoutAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "CLAUDE.md", substantiveAgentInstructions())
	writeFile(t, dir, "CONTRIBUTING.md", "# Contributing\n\n"+strings.Repeat("rule.\n", 25))

	code1, out1, _ := runCLI(t, "assess", "--json", dir)
	code2, out2, _ := runCLI(t, "assess", "--json", dir)
	if code1 != exitOK || code2 != exitOK {
		t.Fatalf("non-zero exit (1=%d, 2=%d)", code1, code2)
	}

	doc1 := scrubTimestamps(t, out1)
	doc2 := scrubTimestamps(t, out2)

	b1, _ := json.Marshal(doc1)
	b2, _ := json.Marshal(doc2)
	if string(b1) != string(b2) {
		t.Errorf("assess --json is not idempotent across runs.\nrun1: %s\nrun2: %s", b1, b2)
	}
}

// ===========================================================================
// SPEC §7 — No-skip rule
// "Verdict.Level is the highest L where every k in 2..L meets the pass
// threshold." A repo with a passing L3 signal but a missing L2 must
// remain at L1 — the climb stops at the first failing level.
// ===========================================================================

func TestIntegration_NoSkipRule_StrongL3WithMissingL2StaysAtL1(t *testing.T) {
	dir := t.TempDir()
	// L3-shaped artifacts (codecov + a coverage gate in CI), no L2 directives.
	writeFile(t, dir, "codecov.yml", "coverage:\n  range: 70..90\n  status:\n    project:\n      default:\n        target: 80%\n")
	writeFile(t, dir, ".github/workflows/ci.yml", "name: CI\non: pull_request\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: pytest --cov-fail-under=70\n")

	code, out, _ := runCLI(t, "assess", "--json", dir)
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	var rpt acmm.Report
	if err := json.Unmarshal([]byte(out), &rpt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rpt.Verdict.Level != acmm.LevelAssisted {
		t.Errorf("Verdict.Level = %d, want 1 (Assisted): the no-skip rule must hold even when L3 has Found signals",
			rpt.Verdict.Level)
	}
}

// ===========================================================================
// SPEC §7 — Verdict invariants
// level_scores values in [0, 1]; Verdict.Level in {1..5}; next_gap
// IDs all reference signals that exist in the registry.
// ===========================================================================

func TestIntegration_VerdictInvariants_LevelScoresBoundedAndNextGapResolves(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n") // bare repo: many missing

	code, out, _ := runCLI(t, "assess", "--json", dir)
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	var rpt acmm.Report
	if err := json.Unmarshal([]byte(out), &rpt); err != nil {
		t.Fatal(err)
	}

	if rpt.Verdict.Level < 1 || rpt.Verdict.Level > 5 {
		t.Errorf("Verdict.Level = %d, want 1..5", rpt.Verdict.Level)
	}
	for lvl, score := range rpt.Verdict.LevelScores {
		if score < 0 || score > 1 {
			t.Errorf("level_scores[L%d] = %f, want in [0, 1]", lvl, score)
		}
	}

	signalIDSet := make(map[string]bool, len(rpt.Signals))
	for _, s := range rpt.Signals {
		signalIDSet[s.ID] = true
	}
	for _, gapID := range rpt.Verdict.NextGap {
		if !signalIDSet[gapID] {
			t.Errorf("next_gap names %q which is not in signals[].id", gapID)
		}
	}
}

// ===========================================================================
// SPEC §6 / §8.2.5 — Stable signal IDs
// All signal IDs match ^l[2-5]\.[a-z0-9-]+$. The format is part of the
// public contract and is what consumers pin in their gating scripts.
// ===========================================================================

var signalIDRE = regexp.MustCompile(`^l[2-5]\.[a-z0-9-]+$`)

func TestIntegration_StableIDFormat_EverySignalIDMatchesContract(t *testing.T) {
	code, out, _ := runCLI(t, "signals", "--json")
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatal(err)
	}
	if len(arr) == 0 {
		t.Fatal("signals --json returned empty array")
	}
	for _, s := range arr {
		id, _ := s["id"].(string)
		if !signalIDRE.MatchString(id) {
			t.Errorf("signal id %q does not match ^l[2-5]\\.[a-z0-9-]+$", id)
		}
	}
}

// ===========================================================================
// SPEC §4 — Exit code matrix
// Every code in §4's table is reachable through a documented path.
// ===========================================================================

func TestIntegration_ExitCodes_GateFailedWhenBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n") // L1

	code, _, _ := runCLI(t, "assess", "--quiet", "--fail-below", "3", dir)
	if code != exitGateFailed {
		t.Errorf("exit = %d, want %d for level=1 below --fail-below=3", code, exitGateFailed)
	}
}

func TestIntegration_ExitCodes_GatePassesWhenAtOrAboveThreshold(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "CLAUDE.md", substantiveAgentInstructions())
	writeFile(t, dir, "CONTRIBUTING.md", "# Contributing\n\n"+strings.Repeat("rule.\n", 25))
	writeFile(t, dir, ".github/pull_request_template.md", "## Summary\n- [ ] x\n- [ ] y\n- [ ] z\n")
	writeFile(t, dir, ".gitmessage", "subject\n\nbody\n")

	code, _, _ := runCLI(t, "assess", "--quiet", "--fail-below", "2", dir)
	if code != exitOK {
		t.Errorf("exit = %d, want %d for level≥2 with --fail-below=2", code, exitOK)
	}
}

func TestIntegration_ExitCodes_CannotRunOnMissingPath(t *testing.T) {
	code, _, errOut := runCLI(t, "assess", "/definitely/not/here/12345")
	if code != exitCannotRun {
		t.Errorf("exit = %d, want %d for missing path; stderr: %s", code, exitCannotRun, errOut)
	}
}

func TestIntegration_ExitCodes_ConfigErrorOnUnknownFlag(t *testing.T) {
	code, _, _ := runCLI(t, "--definitely-not-a-flag")
	if code != exitConfigError {
		t.Errorf("exit = %d, want %d for unknown flag", code, exitConfigError)
	}
}

// ===========================================================================
// SPEC §8.2.7 — --debug routing
// "--debug emits one structured line per probe to stderr. Stdout is
// unchanged — gate output remains stable."
// ===========================================================================

func TestIntegration_DebugFlag_StdoutUnchangedStderrCarriesProbes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	codeA, outA, _ := runCLI(t, "assess", "--json", dir)
	codeB, outB, errB := runCLI(t, "assess", "--json", "--debug", dir)
	if codeA != exitOK || codeB != exitOK {
		t.Fatalf("exit codes a=%d b=%d", codeA, codeB)
	}

	docA := scrubTimestamps(t, outA)
	docB := scrubTimestamps(t, outB)
	bA, _ := json.Marshal(docA)
	bB, _ := json.Marshal(docB)
	if string(bA) != string(bB) {
		t.Errorf("--debug must not alter stdout JSON.\nplain : %s\ndebug : %s", bA, bB)
	}

	if !strings.Contains(errB, "[debug]") {
		t.Errorf("expected [debug] lines on stderr; got:\n%s", errB)
	}
}

// ===========================================================================
// SPEC §8.2.6 — Color suppression
// "Color is suppressed when stdout is not a TTY, --no-color is set, or
// NO_COLOR env is set."
// ===========================================================================

// ansiRE matches CSI sequences (ESC[…). runCLI captures stdout through
// a bytes.Buffer (not a TTY), so the "not a TTY" rule should already
// suppress color. This test pins the more aggressive --no-color case
// because the brief-text output uses status colors at all.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestIntegration_ColorSuppressed_NotATTY(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	// runCLI uses bytes.Buffer (no TTY); per §8.2.6 the output must
	// contain no ANSI escapes regardless of --no-color.
	code, out, _ := runCLI(t, "assess", "--cli", dir)
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if ansiRE.MatchString(out) {
		t.Errorf("ANSI escapes leaked to non-TTY stdout (rule §8.2.6):\n%q", out)
	}
}

// ===========================================================================
// SPEC §8.2.5 — No stdin reads
// "CLI commands never block on stdin. Anything that would prompt in TUI
// mode requires a flag in CLI mode."
// ===========================================================================

func TestIntegration_NoStdinReads_AssessCompletesWithoutStdin(t *testing.T) {
	// runCLI doesn't pipe stdin, so the CLI is reading from /dev/null
	// or the test's terminal. If assess ever reaches a `bufio.NewReader(os.Stdin)`
	// path, this test will hang and the test runner timeout will catch it.
	// Cheap version of the contract assertion: the call returns at all.
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	done := make(chan struct{})
	go func() {
		runCLI(t, "assess", "--json", dir)
		close(done)
	}()
	select {
	case <-done:
		// good — returned without consuming stdin
	case <-time.After(5 * time.Second):
		t.Fatal("assess hung — possible stdin read (violates §8.2.5)")
	}
}

// ===========================================================================
// Cross-format consistency
// The same repo through --json, --report markdown, and the brief text
// summary must agree on the verdict level. (SARIF is currently a stub
// in main, so it's not asserted here.)
// ===========================================================================

func TestIntegration_FormatsAgreeOnVerdict(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "CLAUDE.md", substantiveAgentInstructions())

	codeJSON, outJSON, _ := runCLI(t, "assess", "--json", dir)
	if codeJSON != exitOK {
		t.Fatalf("--json exit = %d", codeJSON)
	}
	var rpt acmm.Report
	_ = json.Unmarshal([]byte(outJSON), &rpt)

	codeMD, outMD, _ := runCLI(t, "assess", "--report", "markdown", "--out", "-", dir)
	if codeMD != exitOK {
		t.Fatalf("--report markdown exit = %d", codeMD)
	}
	wantLevel := "**Level " + intStr(int(rpt.Verdict.Level))
	if !strings.Contains(outMD, wantLevel) {
		t.Errorf("markdown report disagrees with --json verdict level %d; markdown:\n%s",
			rpt.Verdict.Level, outMD)
	}

	codeTxt, outTxt, _ := runCLI(t, "assess", "--cli", dir)
	if codeTxt != exitOK {
		t.Fatalf("brief text exit = %d", codeTxt)
	}
	if !strings.Contains(outTxt, "Assessed level: "+intStr(int(rpt.Verdict.Level))) {
		t.Errorf("brief-text disagrees with --json verdict level %d; text:\n%s",
			rpt.Verdict.Level, outTxt)
	}
}

// ===========================================================================
// SPEC §8.2.5 — --json everywhere
// "signals --json, explain --json, help --json and version --json all
// emit structured output." Verify each is parseable JSON.
// ===========================================================================

func TestIntegration_JSONEverywhere_DocumentedSubcommandsEmitValidJSON(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// arrayShape: top-level `[…]` rather than `{…}`
		arrayShape bool
	}{
		{"signals", []string{"signals", "--json"}, true},
		{"explain", []string{"explain", "l2.agent-instructions", "--json"}, false},
		{"version", []string{"version", "--json"}, false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			code, out, errOut := runCLI(t, c.args...)
			if code != exitOK {
				t.Fatalf("exit = %d (stderr: %s)", code, errOut)
			}
			if c.arrayShape {
				var arr []map[string]any
				if err := json.Unmarshal([]byte(out), &arr); err != nil {
					t.Errorf("%s --json is not valid JSON array: %v\n%s", c.name, err, out)
				}
				return
			}
			var doc map[string]any
			if err := json.Unmarshal([]byte(out), &doc); err != nil {
				t.Errorf("%s --json is not valid JSON object: %v\n%s", c.name, err, out)
			}
		})
	}
}

// ===========================================================================
// SPEC §8.2 — NDJSON event stream
// "--events ndjson emits one JSON object per line on stderr. Each line
// has an `event` field and a monotonic `ts`."
// ===========================================================================

func TestIntegration_EventsNDJSON_EveryLineParseableWithEventField(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	code, _, errOut := runCLI(t, "assess", "--json", "--events", "ndjson", dir)
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if errOut == "" {
		t.Fatal("expected NDJSON events on stderr; got empty")
	}
	lines := strings.Split(strings.TrimSpace(errOut), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected ≥2 event lines; got %d:\n%s", len(lines), errOut)
	}
	for i, line := range lines {
		var evt map[string]any
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			t.Errorf("event line %d not valid JSON: %v\n%s", i, err, line)
			continue
		}
		if _, ok := evt["event"]; !ok {
			t.Errorf("event line %d missing `event` field: %s", i, line)
		}
	}
}

// ===========================================================================
// Quiet mode
// --quiet should suppress the trailing hint and brief-text summary on
// success. (Still emits errors on stderr.)
// ===========================================================================

func TestIntegration_QuietMode_SuppressesHintAndBriefTextOnSuccess(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	codeQuiet, outQuiet, _ := runCLI(t, "assess", "--quiet", dir)
	codeNoisy, outNoisy, _ := runCLI(t, "assess", "--cli", dir)
	if codeQuiet != exitOK || codeNoisy != exitOK {
		t.Fatalf("exit codes quiet=%d noisy=%d", codeQuiet, codeNoisy)
	}
	if len(outQuiet) >= len(outNoisy) {
		t.Errorf("--quiet stdout (%d bytes) should be shorter than non-quiet (%d bytes)\n--quiet:\n%s\n--noisy:\n%s",
			len(outQuiet), len(outNoisy), outQuiet, outNoisy)
	}
}

// ===========================================================================
// Confidence downgrade
// SPEC §7: "Signals scoring < 1.0 at confidence below the gate are
// treated as 0.0 for verdict purposes." A repo whose only signals
// would credit at low confidence should not climb past L1 with
// --min-confidence high.
// ===========================================================================

func TestIntegration_MinConfidence_HighGateDropsPartialCredit(t *testing.T) {
	dir := t.TempDir()
	// CLAUDE.md with a substantive body but no markdown heading.
	// agent_instructions.go scores this as ScoreIncomplete (0.67) at
	// medium confidence — the case the gate is meant to filter:
	// "score < 1.0 at confidence below the gate → treated as 0.0"
	// (SPEC.md §7). The "1.0 honored regardless of confidence"
	// carve-out doesn't apply to partials.
	writeFile(t, dir, "CLAUDE.md", strings.Repeat("rule.\n", 25))

	_, outLow, _ := runCLI(t, "assess", "--json", dir)
	_, outHigh, _ := runCLI(t, "assess", "--json", "--min-confidence", "high", dir)

	var lowRpt, highRpt acmm.Report
	if err := json.Unmarshal([]byte(outLow), &lowRpt); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(outHigh), &highRpt); err != nil {
		t.Fatal(err)
	}

	// The downgrade applies in level_scores, not in the per-signal
	// Score field — the report still reports the raw signal score
	// for transparency, but the verdict math drops the partial.
	lowL2 := lowRpt.Verdict.LevelScores[acmm.LevelInstructed]
	highL2 := highRpt.Verdict.LevelScores[acmm.LevelInstructed]
	if lowL2 <= highL2 {
		t.Errorf("--min-confidence high should reduce level_scores[L2] (partial-credit signal at medium confidence drops to 0); got low=%f high=%f",
			lowL2, highL2)
	}

	if highRpt.Verdict.MinConfidenceApplied != acmm.ConfidenceHigh {
		t.Errorf("min_confidence_applied = %q, want high", highRpt.Verdict.MinConfidenceApplied)
	}
}

func intStr(n int) string { return strconv.Itoa(n) }
