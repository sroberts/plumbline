package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBadge_DefaultWritesSVGIntoRepo covers the zero-flag path: the
// badge lands next to the README it will be referenced from, not in
// the caller's working directory.
func TestBadge_DefaultWritesSVGIntoRepo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	code, out, errOut := runCLI(t, "badge", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}
	if out != "" {
		t.Errorf("expected empty stdout when writing a file, got:\n%s", out)
	}

	body, err := os.ReadFile(filepath.Join(dir, defaultBadgeName))
	if err != nil {
		t.Fatalf("expected %s to exist: %v", defaultBadgeName, err)
	}
	if err := xml.Unmarshal(body, new(struct{})); err != nil {
		t.Fatalf("badge is not well-formed XML: %v", err)
	}
	if !strings.Contains(string(body), "ACMM") {
		t.Errorf("badge missing the ACMM label:\n%s", body)
	}
	if !strings.Contains(errOut, defaultBadgeName) {
		t.Errorf("expected stderr confirmation naming the file, got: %q", errOut)
	}
}

// TestBadge_OutDashStreamsToStdout keeps the badge pipeable and writes
// no file, matching snapshot's --out - behavior.
func TestBadge_OutDashStreamsToStdout(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	code, out, errOut := runCLI(t, "badge", "--out", "-", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "<svg") {
		t.Errorf("expected SVG on stdout, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, defaultBadgeName)); !os.IsNotExist(err) {
		t.Errorf("--out - must not write a file")
	}
}

// TestBadge_LabelOverride lets a repo that already has an "ACMM" badge
// column rename the left side.
func TestBadge_LabelOverride(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	code, out, errOut := runCLI(t, "badge", "--out", "-", "--label", "maturity", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}
	if !strings.Contains(out, "maturity") {
		t.Errorf("badge did not use --label:\n%s", out)
	}
}

// TestBadge_Reproducible is the precondition for the CI drift gate:
// two runs against an unchanged repo must be byte-identical.
func TestBadge_Reproducible(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	_, first, _ := runCLI(t, "badge", "--out", "-", dir)
	_, second, _ := runCLI(t, "badge", "--out", "-", dir)
	if first != second {
		t.Errorf("badge is not reproducible across runs:\n%s\n---\n%s", first, second)
	}
}

// TestBadge_FromSnapshot renders from a committed artifact instead of
// rescanning — the cheap path for a workflow that already has one.
func TestBadge_FromSnapshot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")
	writeFile(t, dir, "CLAUDE.md", strings.Repeat("guidance line\n", 60))

	if code, _, errOut := runCLI(t, "snapshot", dir); code != exitOK {
		t.Fatalf("snapshot exit = %d (%s)", code, errOut)
	}

	artifact := filepath.Join(dir, ".plumbline.toon")
	code, out, errOut := runCLI(t, "badge", "--from", artifact, "--out", "-")
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}
	if !strings.Contains(out, "ACMM") {
		t.Errorf("badge from artifact missing label:\n%s", out)
	}

	// The rendered verdict must match what the artifact recorded, not
	// a fresh scan of whatever directory the CLI happened to run in.
	_, scanned, _ := runCLI(t, "badge", "--out", "-", dir)
	if out != scanned {
		t.Errorf("--from disagrees with a direct scan of the same repo:\n%s\n---\n%s", out, scanned)
	}
}

// TestBadge_FromRejectsPathArg — a path plus --from is ambiguous about
// which repo the badge describes, so refuse rather than silently
// ignoring one of them.
func TestBadge_FromRejectsPathArg(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	code, _, errOut := runCLI(t, "badge", "--from", "x.toon", dir)
	if code != exitCannotRun {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, exitCannotRun, errOut)
	}
}

// TestBadge_FromMissingFile fails loudly rather than emitting an L1
// badge built from a zero-value verdict.
func TestBadge_FromMissingFile(t *testing.T) {
	code, _, errOut := runCLI(t, "badge", "--from", filepath.Join(t.TempDir(), "nope.toon"), "--out", "-")
	if code != exitCannotRun {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, exitCannotRun, errOut)
	}
}

// TestBadge_BadPath surfaces an unreadable repo as "cannot run".
func TestBadge_BadPath(t *testing.T) {
	code, _, errOut := runCLI(t, "badge", filepath.Join(t.TempDir(), "does-not-exist"))
	if code != exitCannotRun {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, exitCannotRun, errOut)
	}
}

// TestBadge_HonorsDisabledSignals proves badge runs the same configured
// pipeline as assess, so a badge can never disagree with the gate beside
// it. The fixture sits at 3-of-4 L2 signals found (0.75, just over the
// 0.7 threshold); disabling one *found* signal drops the average to 0.67
// and the repo to L1, which the badge must reflect.
func TestBadge_HonorsDisabledSignals(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")
	writeFile(t, dir, "CLAUDE.md", "# CLAUDE.md\n\n"+strings.Repeat("guidance line\n", 60))
	writeFile(t, dir, "CONTRIBUTING.md", "# Contributing\n\n"+strings.Repeat("how we work\n", 40))
	writeFile(t, dir, ".github/pull_request_template.md",
		"## Checklist\n\n- [ ] tests\n- [ ] docs\n- [ ] changelog\n")

	_, withSignal, _ := runCLI(t, "badge", "--out", "-", dir)
	if !strings.Contains(withSignal, "L2") {
		t.Fatalf("fixture should assess at L2, got:\n%s", withSignal)
	}

	writeFile(t, dir, ".plumbline.yml", "signals:\n  l2.agent-instructions:\n    enabled: false\n")
	_, withoutSignal, _ := runCLI(t, "badge", "--out", "-", dir)

	if !strings.Contains(withoutSignal, "L1") {
		t.Errorf("disabling a found L2 signal should drop the badge to L1; got:\n%s", withoutSignal)
	}
}
