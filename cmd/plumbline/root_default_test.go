package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// TestRoot_NoSubcommandRunsAssess verifies that `plumbline [path]` (no
// explicit subcommand) drives the assess pipeline — the unified
// interface that runs scan + score + report all at once.
func TestRoot_NoSubcommandRunsAssess(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "CLAUDE.md", substantiveClaudeMD())

	// Bare-invocation form: the path is the only positional arg, no
	// "assess" keyword. --json forces CLI mode (avoiding the TUI).
	code, out, errOut := runCLI(t, "--json", dir)
	if code != exitOK {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	var report acmm.Report
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("bare invocation produced non-JSON stdout: %v\n%s", err, out)
	}
	if report.Schema != "plumbline/v1" {
		t.Errorf("schema = %q, want plumbline/v1", report.Schema)
	}
	if report.Repo == "" {
		t.Errorf("Repo field empty")
	}
}

// TestRoot_BareInvocationEmitsBriefText verifies that `plumbline` with
// no args and no --json (and no TTY in the test environment) prints
// the brief text summary, not the cobra usage page.
func TestRoot_BareInvocationEmitsBriefText(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "CLAUDE.md", substantiveClaudeMD())

	code, out, errOut := runCLI(t, "--cli", dir)
	if code != exitOK {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	for _, want := range []string{"plumbline ·", "Assessed level:", "L2 Instructed"} {
		if !strings.Contains(out, want) {
			t.Errorf("bare invocation stdout missing %q. Got:\n%s", want, out)
		}
	}
}

// TestRoot_SubcommandsStillWork ensures adding a default action to the
// root doesn't shadow the explicit subcommands.
func TestRoot_SubcommandsStillWork(t *testing.T) {
	for _, args := range [][]string{
		{"signals", "--json"},
		{"version"},
		{"help"},
		{"explain", "l2.claude-md"},
	} {
		args := args
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, _, errOut := runCLI(t, args...)
			if code != exitOK {
				t.Errorf("exit = %d, want 0 for %v (stderr: %s)", code, args, errOut)
			}
		})
	}
}
