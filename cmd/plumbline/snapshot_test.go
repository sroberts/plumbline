package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// TestSnapshot_DefaultWritesToonIntoRepo verifies the zero-flag path:
// `plumbline snapshot <repo>` drops a .plumbline.toon inside <repo> (not
// the caller's working directory) and confirms on stderr.
func TestSnapshot_DefaultWritesToonIntoRepo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	code, out, errOut := runCLI(t, "snapshot", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}
	if out != "" {
		t.Errorf("expected empty stdout when writing a file, got:\n%s", out)
	}

	artifact := filepath.Join(dir, ".plumbline.toon")
	body, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", artifact, err)
	}
	if !strings.Contains(string(body), "schema: plumbline/v1") {
		t.Errorf("TOON artifact missing schema line:\n%s", body)
	}
	if !strings.Contains(errOut, ".plumbline.toon") {
		t.Errorf("expected stderr confirmation naming the file, got: %q", errOut)
	}
}

// TestSnapshot_FormatSelectsExtension checks --format json|yaml chooses
// the matching default filename and emits decodable content.
func TestSnapshot_FormatSelectsExtension(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "README.md", "# r\n")

		code, _, errOut := runCLI(t, "snapshot", "--format", "json", dir)
		if code != exitOK {
			t.Fatalf("exit = %d (stderr: %s)", code, errOut)
		}
		body, err := os.ReadFile(filepath.Join(dir, ".plumbline.json"))
		if err != nil {
			t.Fatalf("expected .plumbline.json: %v", err)
		}
		var rpt acmm.Report
		if err := json.Unmarshal(body, &rpt); err != nil {
			t.Fatalf("json artifact does not parse: %v", err)
		}
		if rpt.Schema != "plumbline/v1" {
			t.Errorf("schema = %q, want plumbline/v1", rpt.Schema)
		}
	})

	t.Run("yaml", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "README.md", "# r\n")

		code, _, errOut := runCLI(t, "snapshot", "--format", "yaml", dir)
		if code != exitOK {
			t.Fatalf("exit = %d (stderr: %s)", code, errOut)
		}
		body, err := os.ReadFile(filepath.Join(dir, ".plumbline.yaml"))
		if err != nil {
			t.Fatalf("expected .plumbline.yaml: %v", err)
		}
		var doc map[string]any
		if err := yaml.Unmarshal(body, &doc); err != nil {
			t.Fatalf("yaml artifact does not parse: %v", err)
		}
		if doc["schema"] != "plumbline/v1" {
			t.Errorf("schema = %v, want plumbline/v1", doc["schema"])
		}
	})
}

// TestSnapshot_StdoutViaDashDoesNotWriteFile confirms --out - streams the
// artifact to stdout and leaves no dotfile behind.
func TestSnapshot_StdoutViaDashDoesNotWriteFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	code, out, errOut := runCLI(t, "snapshot", "--out", "-", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}
	if !strings.Contains(out, "schema: plumbline/v1") {
		t.Errorf("expected TOON on stdout, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".plumbline.toon")); !os.IsNotExist(err) {
		t.Errorf("expected no .plumbline.toon written when --out -, stat err = %v", err)
	}
	// No confirmation line when the artifact itself went to stdout.
	if strings.Contains(errOut, "wrote") {
		t.Errorf("did not expect a 'wrote' confirmation for stdout, got: %q", errOut)
	}
}

func TestSnapshot_InvalidFormat(t *testing.T) {
	dir := t.TempDir()
	code, _, errOut := runCLI(t, "snapshot", "--format", "xml", dir)
	if code != exitCannotRun {
		t.Fatalf("exit = %d, want %d", code, exitCannotRun)
	}
	if !strings.Contains(errOut, "invalid --format") {
		t.Errorf("expected invalid-format error, got: %q", errOut)
	}
}

// Signals disabled in .plumbline.yml must not appear in the artifact —
// snapshot honors config exactly like assess.
func TestSnapshot_HonorsDisabledSignals(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")
	writeFile(t, dir, ".plumbline.yml", "signals:\n  l3.user-feedback:\n    enabled: false\n")

	code, _, errOut := runCLI(t, "snapshot", "--format", "json", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}
	body, err := os.ReadFile(filepath.Join(dir, ".plumbline.json"))
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	var rpt acmm.Report
	if err := json.Unmarshal(body, &rpt); err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	for _, s := range rpt.Signals {
		if s.ID == "l3.user-feedback" {
			t.Errorf("disabled signal l3.user-feedback should not be in the snapshot")
		}
	}
}
