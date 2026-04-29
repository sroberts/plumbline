package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSARIFReport_EndToEnd makes sure --report sarif emits a parseable
// SARIF 2.1.0 document with at least one rule + result for a missing
// signal. The unit tests in internal/report/sarif_test.go cover the
// shape; this exercises the wiring through writeReport.
func TestSARIFReport_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	code, out, errOut := runCLI(t, "assess", "--report", "sarif", "--out", "-", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}

	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, out)
	}
	if doc["version"] != "2.1.0" {
		t.Errorf("version = %v, want 2.1.0", doc["version"])
	}

	// A bare repo with only README.md is missing l2.agent-instructions —
	// that should show up as a SARIF result.
	if !strings.Contains(out, `"l2.agent-instructions"`) {
		t.Errorf("expected missing signal id in SARIF results, got:\n%s", out)
	}
	if !strings.Contains(out, `"error"`) {
		t.Errorf("expected at least one error-severity finding, got:\n%s", out)
	}
}
