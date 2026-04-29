package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sroberts/plumbline/pkg/acmm"
)

const samplePluginOutput = `{"id":"x.demo","level":3,"family":"custom","title":"Demo plugin","status":"missing","score":0,"confidence":"medium","method":"filename","fix_hint":"add a thing"}`

func writeTestPlugin(t *testing.T, stdout string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("plugin e2e uses /bin/sh; skip on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "plugin.sh")
	body := "#!/bin/sh\ncat <<'EOF'\n" + stdout + "\nEOF\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPlugin_AppearsInVerdictAlongsideBuiltins(t *testing.T) {
	repoDir := t.TempDir()
	writeFile(t, repoDir, "README.md", "# r\n")

	plug := writeTestPlugin(t, samplePluginOutput)
	code, out, errOut := runCLI(t, "assess", "--json", "--plugin", plug, repoDir)
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}
	var rpt acmm.Report
	if err := json.Unmarshal([]byte(out), &rpt); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	found := false
	for _, s := range rpt.Signals {
		if s.ID == "x.demo" {
			found = true
			if s.Status != acmm.StatusMissing {
				t.Errorf("plugin signal status = %q, want missing", s.Status)
			}
			if s.FixHint != "add a thing" {
				t.Errorf("FixHint not carried through: %q", s.FixHint)
			}
		}
	}
	if !found {
		ids := make([]string, len(rpt.Signals))
		for i, s := range rpt.Signals {
			ids[i] = s.ID
		}
		t.Errorf("plugin signal x.demo not in report; got ids=%v", ids)
	}
}

func TestPlugin_FailingPluginFailsAssess(t *testing.T) {
	repoDir := t.TempDir()
	writeFile(t, repoDir, "README.md", "# r\n")

	dir := t.TempDir()
	path := filepath.Join(dir, "broken.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	code, _, errOut := runCLI(t, "assess", "--plugin", path, repoDir)
	if code != exitCannotRun {
		t.Errorf("exit = %d, want %d (stderr: %s)", code, exitCannotRun, errOut)
	}
	if !strings.Contains(errOut, "plugin probe") {
		t.Errorf("expected probe-failure context in error; got:\n%s", errOut)
	}
}
