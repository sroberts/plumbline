package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistoryOut_AppendsOnePerInvocation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")
	hist := filepath.Join(dir, "maturity.ndjson")

	for i := 0; i < 3; i++ {
		code, _, errOut := runCLI(t, "assess", "--quiet",
			"--history-out", hist, dir)
		if code != exitOK {
			t.Fatalf("assess #%d exit = %d (stderr: %s)", i, code, errOut)
		}
	}

	data, err := os.ReadFile(hist)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (after 3 invocations):\n%s", len(lines), data)
	}
	for i, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d not valid JSON: %v\n%s", i, err, line)
			continue
		}
		if _, ok := entry["level"]; !ok {
			t.Errorf("line %d missing level field: %s", i, line)
		}
		if _, ok := entry["status_counts"]; !ok {
			t.Errorf("line %d missing status_counts field: %s", i, line)
		}
	}
}

func TestHistoryOut_FailureDoesNotKillAssess(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")
	// Point --history-out at a path whose parent directory doesn't exist.
	// A history-write failure must not propagate to the assess exit code —
	// otherwise CI ratchets break when /tmp fills up.
	hist := filepath.Join(dir, "no", "such", "dir", "history.ndjson")

	code, _, errOut := runCLI(t, "assess", "--history-out", hist, dir)
	if code != exitOK {
		t.Errorf("assess exit = %d, want %d (stderr: %s)", code, exitOK, errOut)
	}
	if !strings.Contains(errOut, "history append failed") {
		t.Errorf("expected warning on stderr; got:\n%s", errOut)
	}
}
