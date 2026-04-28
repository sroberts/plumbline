package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// writeFile is a small helper to write a file at <dir>/<rel>.
func writeFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// substantiveAgentInstructions is a CLAUDE.md body that satisfies the
// l2.agent-instructions signal (heading + ≥20 non-blank lines).
func substantiveAgentInstructions() string {
	body := "# CLAUDE.md\n\n"
	body += strings.Repeat("Some real, non-blank line of guidance.\n", 25)
	return body
}

func TestAssessJSON_FoundAllL2ReachesLevelTwo(t *testing.T) {
	dir := t.TempDir()
	// Write a substantive artifact for every L2 signal so the level
	// passes the threshold. ONE agent-instructions file is enough now —
	// the signal is satisfied by any of CLAUDE.md / AGENTS.md /
	// .github/copilot-instructions.md / .cursorrules / .windsurfrules.
	writeFile(t, dir, "CLAUDE.md", substantiveAgentInstructions())
	writeFile(t, dir, "CONTRIBUTING.md", "# Contributing\n\n"+strings.Repeat("rule.\n", 25))
	writeFile(t, dir, ".github/pull_request_template.md", "## Summary\n\n- [ ] one\n- [ ] two\n- [ ] three\n")
	writeFile(t, dir, ".gitmessage", "subject\n\nbody\n")

	code, out, errOut := runCLI(t, "assess", "--json", dir)
	if code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, exitOK, errOut)
	}

	var report acmm.Report
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("unmarshal: %v\nstdout: %s", err, out)
	}

	if report.Schema != "plumbline/v1" {
		t.Errorf("schema = %q, want plumbline/v1", report.Schema)
	}
	if report.SignalSetVersion != "v1" {
		t.Errorf("signal_set_version = %q, want v1", report.SignalSetVersion)
	}
	if report.Verdict.Level != acmm.LevelInstructed {
		t.Errorf("Verdict.Level = %d, want 2 (Instructed) for substantive CLAUDE.md", report.Verdict.Level)
	}
	if len(report.Signals) == 0 {
		t.Fatal("Signals is empty")
	}

	var got *acmm.SignalResult
	for i := range report.Signals {
		if report.Signals[i].ID == "l2.agent-instructions" {
			got = &report.Signals[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("l2.agent-instructions not in signals; ids=%v", signalIDs(report.Signals))
	}
	if got.Status != acmm.StatusFound {
		t.Errorf("l2.agent-instructions status = %q, want found", got.Status)
	}
	if got.Score != acmm.ScoreFound {
		t.Errorf("l2.agent-instructions score = %v, want %v", got.Score, acmm.ScoreFound)
	}
}

// TestAssessJSON_AGENTSmdAlsoCounts is the user's whole point — having
// AGENTS.md (or .cursorrules, etc.) instead of CLAUDE.md is enough for
// the L2 agent-instructions signal.
func TestAssessJSON_AGENTSmdAlsoCounts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "AGENTS.md", "# AGENTS\n\n"+strings.Repeat("rule.\n", 25))

	code, out, _ := runCLI(t, "assess", "--json", dir)
	if code != exitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	var report acmm.Report
	_ = json.Unmarshal([]byte(out), &report)

	for _, s := range report.Signals {
		if s.ID == "l2.agent-instructions" {
			if s.Score != acmm.ScoreFound {
				t.Errorf("with AGENTS.md only, score = %v, want Found", s.Score)
			}
			return
		}
	}
	t.Fatal("l2.agent-instructions not present in report")
}

func TestAssessJSON_MissingAgentInstructionsStaysAtLevelOne(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# repo\n")

	code, out, errOut := runCLI(t, "assess", "--json", dir)
	if code != exitOK {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, exitOK, errOut)
	}

	var report acmm.Report
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("unmarshal: %v\nstdout: %s", err, out)
	}

	if report.Verdict.Level != acmm.LevelAssisted {
		t.Errorf("Verdict.Level = %d, want 1 (Assisted) when no agent file is present", report.Verdict.Level)
	}
	found := false
	for _, id := range report.Verdict.NextGap {
		if id == "l2.agent-instructions" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("next_gap missing 'l2.agent-instructions'; got %v", report.Verdict.NextGap)
	}
}

func TestAssessJSON_BadPathExitsCannotRun(t *testing.T) {
	code, _, errOut := runCLI(t, "assess", "--json", "/definitely/does/not/exist/12345")
	if code != exitCannotRun {
		t.Errorf("exit = %d, want %d (cannot run)", code, exitCannotRun)
	}
	if !strings.Contains(errOut, "error:") {
		t.Errorf("expected an 'error:' line in stderr, got: %s", errOut)
	}
}

func signalIDs(sigs []acmm.SignalResult) []string {
	out := make([]string, len(sigs))
	for i, s := range sigs {
		out[i] = s.ID
	}
	return out
}
