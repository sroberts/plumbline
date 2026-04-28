package main

import (
	"bytes"
	"strings"
	"testing"
)

// runCLI is a test helper that invokes the CLI in-process.
func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRoot_NoArgsShowsHelp(t *testing.T) {
	code, out, _ := runCLI(t)
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d", exitOK, code)
	}
	if !strings.Contains(out, "plumbline assesses") {
		t.Errorf("root help missing long-description marker; got:\n%s", out)
	}
	if !strings.Contains(out, "Available Commands") {
		t.Errorf("root help missing command list; got:\n%s", out)
	}
}

func TestVersion_PlainText(t *testing.T) {
	code, out, _ := runCLI(t, "version")
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d", exitOK, code)
	}
	if !strings.Contains(out, "plumbline ") {
		t.Errorf("expected version banner, got:\n%s", out)
	}
	if !strings.Contains(out, "signal-set: v1") {
		t.Errorf("expected signal-set version line, got:\n%s", out)
	}
}

func TestVersion_JSON(t *testing.T) {
	code, out, _ := runCLI(t, "version", "--json")
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d", exitOK, code)
	}
	if !strings.Contains(out, `"signal_set_version": "v1"`) {
		t.Errorf("expected signal_set_version field in JSON, got:\n%s", out)
	}
}

func TestHelp_TopicIndex(t *testing.T) {
	code, out, _ := runCLI(t, "help")
	if code != exitOK {
		t.Fatalf("expected exit %d, got %d", exitOK, code)
	}
	for _, topic := range []string{"levels", "signals", "scoring", "agents", "compatibility"} {
		if !strings.Contains(out, "plumbline help "+topic) {
			t.Errorf("help index missing topic %q; got:\n%s", topic, out)
		}
	}
}

func TestHelp_UnknownTopic(t *testing.T) {
	code, _, errOut := runCLI(t, "help", "made-up-topic")
	if code != exitCannotRun {
		t.Fatalf("expected exit %d for unknown topic, got %d (stderr: %s)", exitCannotRun, code, errOut)
	}
	if !strings.Contains(errOut, "unknown topic") {
		t.Errorf("expected 'unknown topic' in stderr, got:\n%s", errOut)
	}
}

// TestImplementedCommands_ExitOK is the inverse of the old stub test.
// All commands the spec covers are wired; this asserts they exit 0 in
// their happy paths.
func TestImplementedCommands_ExitOK(t *testing.T) {
	cases := [][]string{
		{"signals"},
		{"signals", "--json"},
		{"signals", "--level", "2"},
		{"explain", "l2.claude-md"},
		{"explain", "l2.claude-md", "--json"},
		{"schema", "verdict"},
		{"schema", "signal-result"},
		{"schema", "event"},
		{"schema", "config"},
		{"version"},
		{"version", "--json"},
		{"help"},
		{"help", "agents"},
		{"help", "scoring"},
	}
	for _, args := range cases {
		args := args
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, _, errOut := runCLI(t, args...)
			if code != exitOK {
				t.Errorf("exit = %d, want 0 (stderr: %s)", code, errOut)
			}
		})
	}
}

func TestUnknownSignal_ExitsCannotRun(t *testing.T) {
	code, _, errOut := runCLI(t, "explain", "definitely-not-a-signal")
	if code != exitCannotRun {
		t.Errorf("exit = %d, want %d (stderr: %s)", code, exitCannotRun, errOut)
	}
	if !strings.Contains(errOut, "unknown signal") {
		t.Errorf("expected 'unknown signal' in stderr, got: %s", errOut)
	}
}

func TestUnknownSchema_ExitsCannotRun(t *testing.T) {
	code, _, errOut := runCLI(t, "schema", "definitely-not-a-schema")
	// cobra's ValidArgs check catches this before our RunE; either exit code is acceptable.
	if code == exitOK {
		t.Errorf("expected non-zero exit for unknown schema (stderr: %s)", errOut)
	}
}

// TestAssess_FlagValidation checks the cross-flag rules that hold
// independent of milestone (mutually exclusive --cli/--tui, etc.).
func TestAssess_FlagValidation(t *testing.T) {
	code, _, errOut := runCLI(t, "assess", "--cli", "--tui")
	if code != exitCannotRun {
		t.Errorf("expected exit %d, got %d (stderr: %s)", exitCannotRun, code, errOut)
	}
	if !strings.Contains(errOut, "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in stderr, got:\n%s", errOut)
	}
}

func TestUnknownCommand_ExitsConfigError(t *testing.T) {
	code, _, _ := runCLI(t, "definitely-not-a-command")
	if code != exitConfigError {
		t.Errorf("expected exit %d for unknown command, got %d", exitConfigError, code)
	}
}
