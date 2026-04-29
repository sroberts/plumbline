// End-to-end coverage for the v1 → v2 deprecation aliases. The unit
// tests in internal/signals/aliases_test.go cover the resolver in
// isolation; these exercise the full assess pipeline so a regression
// in flag wiring or warning routing trips here.
package main

import (
	"strings"
	"testing"
)

func TestAlias_DeprecatedIncludeSignalRewritesAndWarns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "CLAUDE.md", "# c\n\n"+strings.Repeat("rule.\n", 25))

	// Pass the v1 alias l2.claude-md. It should rewrite to
	// l2.agent-instructions and the assess should still find the
	// CLAUDE.md → Found verdict for that signal.
	code, out, errOut := runCLI(t, "assess", "--json", "--include-signal", "l2.claude-md", dir)
	if code != exitOK {
		t.Fatalf("exit = %d; stderr: %s", code, errOut)
	}
	if !strings.Contains(out, `"l2.agent-instructions"`) {
		t.Errorf("verdict JSON should include rewritten signal id; got:\n%s", out)
	}
	if !strings.Contains(errOut, "deprecated since signal-set") {
		t.Errorf("expected deprecation warning on stderr; got:\n%s", errOut)
	}
	if !strings.Contains(errOut, "l2.claude-md") {
		t.Errorf("warning should name the deprecated id; got:\n%s", errOut)
	}
}

func TestAlias_QuietSuppressesDeprecationWarning(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "CLAUDE.md", "# c\n\n"+strings.Repeat("rule.\n", 25))

	// --quiet routes warnings to io.Discard so JSON-only consumers
	// stay machine-clean even when their pinned IDs are deprecated.
	code, _, errOut := runCLI(t, "assess", "--quiet", "--json",
		"--include-signal", "l2.copilot-instructions", dir)
	if code != exitOK {
		t.Fatalf("exit = %d; stderr: %s", code, errOut)
	}
	if strings.Contains(errOut, "deprecated") {
		t.Errorf("--quiet should suppress deprecation warning; got:\n%s", errOut)
	}
}

func TestAlias_UnknownSignalSetRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r")

	// v99 is not a known signal-set version; assess should refuse
	// rather than silently fall back. Exit code 2 (cannot run) is
	// the right shape per SPEC.md §4.
	code, _, errOut := runCLI(t, "assess", "--signal-set", "v99", dir)
	if code != exitCannotRun {
		t.Errorf("exit = %d, want %d (stderr: %s)", code, exitCannotRun, errOut)
	}
	if !strings.Contains(errOut, "unknown --signal-set") {
		t.Errorf("expected unknown-signal-set error, got:\n%s", errOut)
	}
}

func TestAlias_PinnedToV1StillAccepted(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r")

	// CI gates pinned to --signal-set v1 must keep working through
	// the deprecation cycle. Otherwise the "stable IDs" promise in
	// SPEC.md §8.2.5 has a one-release escape hatch.
	code, _, errOut := runCLI(t, "assess", "--signal-set", "v1", "--cli", dir)
	if code != exitOK {
		t.Errorf("exit = %d, want %d (stderr: %s)", code, exitOK, errOut)
	}
}
