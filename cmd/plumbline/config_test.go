package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sroberts/plumbline/pkg/acmm"
)

func TestConfig_DisabledSignalDropsFromVerdict(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")
	writeFile(t, dir, ".plumbline.yml", `signals:
  l3.user-feedback:
    enabled: false
`)

	code, out, errOut := runCLI(t, "assess", "--json", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}
	var rpt acmm.Report
	if err := json.Unmarshal([]byte(out), &rpt); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	for _, s := range rpt.Signals {
		if s.ID == "l3.user-feedback" {
			t.Errorf("l3.user-feedback present despite enabled:false in config")
		}
	}
}

func TestConfig_UnknownKeyExitsWithError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")
	writeFile(t, dir, ".plumbline.yml", "typo_at_top_level: oops\n")

	code, _, errOut := runCLI(t, "assess", "--json", dir)
	if code != exitCannotRun {
		t.Errorf("exit = %d, want %d (stderr: %s)", code, exitCannotRun, errOut)
	}
	if !strings.Contains(errOut, "typo_at_top_level") {
		t.Errorf("expected typo'd key name in error; got:\n%s", errOut)
	}
}

func TestConfig_ExplicitConfigPathMissingFileIsError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	code, _, errOut := runCLI(t, "assess", "--config",
		dir+"/no-such-config.yml", dir)
	if code != exitCannotRun {
		t.Errorf("exit = %d, want %d (stderr: %s)", code, exitCannotRun, errOut)
	}
}

func TestConfig_AbsentDefaultDoesNotErrorBareRepo(t *testing.T) {
	// A repo with no .plumbline.yml must still assess cleanly. Otherwise
	// the optional-config promise in helpConfig is broken.
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	code, _, errOut := runCLI(t, "assess", "--json", dir)
	if code != exitOK {
		t.Errorf("bare repo (no .plumbline.yml) exit = %d, want %d (stderr: %s)",
			code, exitOK, errOut)
	}
}
