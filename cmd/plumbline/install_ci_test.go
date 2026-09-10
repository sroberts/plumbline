package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/sroberts/plumbline/internal/ciworkflow"
)

// TestInstallCI_DryRunByDefault — writing CI configuration without
// being asked is exactly the surprise `fix` and `install-skill` are
// careful to avoid.
func TestInstallCI_DryRunByDefault(t *testing.T) {
	dir := t.TempDir()

	code, out, errOut := runCLI(t, "install-ci", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}
	if !strings.Contains(out, "DRY-RUN") {
		t.Errorf("expected a DRY-RUN banner, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, ciworkflow.DefaultPath)); !os.IsNotExist(err) {
		t.Errorf("dry run must not write the workflow")
	}
}

// TestInstallCI_ApplyWritesValidWorkflow is the happy path.
func TestInstallCI_ApplyWritesValidWorkflow(t *testing.T) {
	dir := t.TempDir()

	code, _, errOut := runCLI(t, "install-ci", "--apply", "--fail-below", "3", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}

	body, err := os.ReadFile(filepath.Join(dir, ciworkflow.DefaultPath))
	if err != nil {
		t.Fatalf("expected %s: %v", ciworkflow.DefaultPath, err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("installed workflow is not valid YAML: %v\n%s", err, body)
	}
	if !strings.Contains(string(body), "--fail-below 3") {
		t.Errorf("--fail-below not threaded into the workflow:\n%s", body)
	}
}

// TestInstallCI_VariantSelects covers the picker's CLI equivalent.
func TestInstallCI_VariantSelects(t *testing.T) {
	dir := t.TempDir()

	code, _, errOut := runCLI(t, "install-ci", "--variant", "badge", "--apply", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}
	body, err := os.ReadFile(filepath.Join(dir, ciworkflow.DefaultPath))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "--fail-below") {
		t.Errorf("badge variant should not gate on level:\n%s", body)
	}
	if !strings.Contains(string(body), "plumbline badge") {
		t.Errorf("badge variant missing the badge step:\n%s", body)
	}
}

// TestInstallCI_RefusesOverwrite — a repo's existing CI config is not
// ours to replace.
func TestInstallCI_RefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ciworkflow.DefaultPath, "name: mine\n")

	code, _, errOut := runCLI(t, "install-ci", "--apply", dir)
	if code != exitCannotRun {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, exitCannotRun, errOut)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ciworkflow.DefaultPath))
	if string(body) != "name: mine\n" {
		t.Errorf("existing workflow was modified: %q", body)
	}
}

// TestInstallCI_OutOverride lets a repo install alongside an existing
// plumbline.yml, or keep a different filename.
func TestInstallCI_OutOverride(t *testing.T) {
	dir := t.TempDir()

	code, _, errOut := runCLI(t, "install-ci", "--out", ".github/workflows/acmm.yml", "--apply", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, ".github/workflows/acmm.yml")); err != nil {
		t.Errorf("--out ignored: %v", err)
	}
}

// TestInstallCI_List prints the variants without touching the repo.
func TestInstallCI_List(t *testing.T) {
	code, out, errOut := runCLI(t, "install-ci", "--list")
	if code != exitOK {
		t.Fatalf("exit = %d (stderr: %s)", code, errOut)
	}
	for _, v := range ciworkflow.Variants() {
		if !strings.Contains(out, v.ID) {
			t.Errorf("--list missing variant %q:\n%s", v.ID, out)
		}
	}
}

// TestInstallCI_RejectsBadInput checks the validation surface reaches
// the CLI's exit-code contract.
func TestInstallCI_RejectsBadInput(t *testing.T) {
	dir := t.TempDir()

	if code, _, _ := runCLI(t, "install-ci", "--variant", "nope", dir); code != exitCannotRun {
		t.Errorf("unknown variant exit = %d, want %d", code, exitCannotRun)
	}
	if code, _, _ := runCLI(t, "install-ci", "--fail-below", "9", dir); code != exitCannotRun {
		t.Errorf("out-of-range --fail-below exit = %d, want %d", code, exitCannotRun)
	}
}

// TestInstallCI_CannotBootstrapALevel guards the boundary SPEC.md §4
// draws around this command: plumbline must never score a repo for a file
// plumbline wrote.
//
// It used to assert a weaker thing — that the scaffolded workflow scored
// `partial` but could not carry a level. That partial credit turned out
// to be a detector bug: the workflow's only build-shaped line was the
// `go install <module>@latest` that fetches plumbline, and a remote tool
// fetch builds nothing belonging to the repo. With that excluded, the
// workflow earns nothing, and both halves are worth pinning.
//
// The fixture is an L2 repo on purpose. An L1 repo cannot move whatever
// an L3 signal does — levels are sequential, so unmet L2 pins the verdict
// at 1 and the assertion would hold for a reason unrelated to the
// behavior under test. This test asserted exactly that vacuous thing
// until code review caught it. From L2, credit at L3 is genuinely the
// kind of thing that could push a verdict, so the assertion has teeth.
func TestInstallCI_CannotBootstrapALevel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")
	writeFile(t, dir, "CLAUDE.md", "# CLAUDE.md\n\n"+strings.Repeat("guidance line\n", 60))
	writeFile(t, dir, "CONTRIBUTING.md", "# Contributing\n\n"+strings.Repeat("how we work\n", 40))
	writeFile(t, dir, ".github/pull_request_template.md",
		"## Checklist\n\n- [ ] tests\n- [ ] docs\n- [ ] changelog\n")
	writeFile(t, dir, ".gitmessage", "subject\n\n# Conventional commits: feat|fix|docs\n")

	_, before, _ := runCLI(t, "assess", "--report", "json", dir)
	if lvl := levelOf(t, before); lvl != 2 {
		t.Fatalf("fixture must start at L2 for this test to mean anything, got L%d", lvl)
	}
	if got := signalStatus(t, before, "l3.build-lint-gate"); got != "missing" {
		t.Fatalf("fixture should start with no build/lint gate, got %q", got)
	}

	if code, _, errOut := runCLI(t, "install-ci", "--apply", "--fail-below", "3", dir); code != exitOK {
		t.Fatalf("install-ci exit = %d (%s)", code, errOut)
	}

	_, after, _ := runCLI(t, "assess", "--report", "json", dir)

	if got := signalStatus(t, after, "l3.build-lint-gate"); got != "missing" {
		t.Errorf("l3.build-lint-gate = %q after installing plumbline's own workflow, want missing; "+
			"the workflow installs plumbline and runs it — it does not build or lint this repo, "+
			"and crediting it would be plumbline scoring a file plumbline wrote", got)
	}
	if lvl := levelOf(t, after); lvl != 2 {
		t.Errorf("installing plumbline's own workflow moved the verdict from L2 to L%d", lvl)
	}
}

// signalStatus pulls one signal's status out of an assess --report json
// payload.
func signalStatus(t *testing.T, jsonReport, id string) string {
	t.Helper()
	var doc struct {
		Signals []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"signals"`
	}
	if err := json.Unmarshal([]byte(jsonReport), &doc); err != nil {
		t.Fatalf("parse report: %v", err)
	}
	for _, sig := range doc.Signals {
		if sig.ID == id {
			return sig.Status
		}
	}
	t.Fatalf("signal %q not present in report", id)
	return ""
}

// levelOf pulls verdict.level out of an assess --report json payload.
func levelOf(t *testing.T, jsonReport string) int {
	t.Helper()
	var doc struct {
		Verdict struct {
			Level int `json:"level"`
		} `json:"verdict"`
	}
	if err := json.Unmarshal([]byte(jsonReport), &doc); err != nil {
		t.Fatalf("parse report: %v", err)
	}
	return doc.Verdict.Level
}
