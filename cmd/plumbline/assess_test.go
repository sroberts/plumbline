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

// substantiveClaudeMD is a CLAUDE.md body that satisfies the L2 signal
// (heading + ≥30 non-blank lines).
func substantiveClaudeMD() string {
	body := "# CLAUDE.md\n\n"
	body += strings.Repeat("Some real, non-blank line of guidance.\n", 35)
	return body
}

func TestAssessJSON_FoundAllL2ReachesLevelTwo(t *testing.T) {
	dir := t.TempDir()
	// Write a substantive artifact for every L2 signal so the level
	// passes the threshold.
	writeFile(t, dir, "CLAUDE.md", substantiveClaudeMD())
	writeFile(t, dir, ".github/copilot-instructions.md", "# Copilot\n\n"+strings.Repeat("guideline.\n", 25))
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
		if report.Signals[i].ID == "l2.claude-md" {
			got = &report.Signals[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("l2.claude-md not in signals; ids=%v", signalIDs(report.Signals))
	}
	if got.Status != acmm.StatusFound {
		t.Errorf("l2.claude-md status = %q, want found", got.Status)
	}
	if got.Score != acmm.ScoreFound {
		t.Errorf("l2.claude-md score = %v, want %v", got.Score, acmm.ScoreFound)
	}
}

func TestAssessJSON_MissingClaudeMDStaysAtLevelOne(t *testing.T) {
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
		t.Errorf("Verdict.Level = %d, want 1 (Assisted) when CLAUDE.md is missing", report.Verdict.Level)
	}
	// next_gap should call out the missing signal at L+1 (=L2).
	found := false
	for _, id := range report.Verdict.NextGap {
		if id == "l2.claude-md" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("next_gap missing 'l2.claude-md'; got %v", report.Verdict.NextGap)
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
