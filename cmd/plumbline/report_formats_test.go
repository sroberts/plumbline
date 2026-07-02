package main

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestAssessReportTOON_EndToEnd exercises the --report toon wiring
// through writeReport; the encoder itself is unit-tested in
// internal/report.
func TestAssessReportTOON_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	code, out, errOut := runCLI(t, "assess", "--report", "toon", "--out", "-", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}
	if !strings.Contains(out, "schema: plumbline/v1") {
		t.Errorf("expected TOON output, got:\n%s", out)
	}
	if !strings.Contains(out, "verdict:") {
		t.Errorf("expected verdict block in TOON, got:\n%s", out)
	}
}

func TestAssessReportYAML_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	code, out, errOut := runCLI(t, "assess", "--report", "yaml", "--out", "-", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not valid YAML: %v\n%s", err, out)
	}
	if doc["schema"] != "plumbline/v1" {
		t.Errorf("schema = %v, want plumbline/v1", doc["schema"])
	}
}

func TestAssessReport_InvalidFormat(t *testing.T) {
	dir := t.TempDir()
	code, _, errOut := runCLI(t, "assess", "--report", "xml", "--out", "-", dir)
	if code != exitCannotRun {
		t.Fatalf("exit = %d, want %d", code, exitCannotRun)
	}
	if !strings.Contains(errOut, "want json|markdown|sarif|toon|yaml") {
		t.Errorf("expected format-list error, got: %q", errOut)
	}
}
