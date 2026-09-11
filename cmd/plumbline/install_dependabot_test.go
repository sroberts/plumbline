package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/sroberts/plumbline/internal/dependabot"
)

func TestInstallDependabot_DryRunByDefault(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n")

	code, out, errOut := runCLI(t, "install-dependabot", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (%s)", code, errOut)
	}
	if !strings.Contains(out, "DRY-RUN") {
		t.Errorf("expected a DRY-RUN banner:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, dependabot.DefaultPath)); !os.IsNotExist(err) {
		t.Errorf("dry run must not write the config")
	}
}

// TestInstallDependabot_ApplyWritesValidConfig — a malformed config makes
// GitHub silently disable updates, so validity is the load-bearing check.
func TestInstallDependabot_ApplyWritesValidConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n")
	writeFile(t, dir, ".github/workflows/ci.yml", "name: CI\non: [push]\njobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: go build ./...\n")

	code, _, errOut := runCLI(t, "install-dependabot", "--apply", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (%s)", code, errOut)
	}
	body, err := os.ReadFile(filepath.Join(dir, dependabot.DefaultPath))
	if err != nil {
		t.Fatalf("expected %s: %v", dependabot.DefaultPath, err)
	}
	var doc struct {
		Version int `yaml:"version"`
		Updates []struct {
			PackageEcosystem string `yaml:"package-ecosystem"`
		} `yaml:"updates"`
	}
	if err := yaml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("config is not valid YAML: %v\n%s", err, body)
	}
	if doc.Version != 2 {
		t.Errorf("version = %d, want 2", doc.Version)
	}
	var eco []string
	for _, u := range doc.Updates {
		eco = append(eco, u.PackageEcosystem)
	}
	if strings.Join(eco, ",") != "github-actions,gomod" {
		t.Errorf("ecosystems = %v, want [github-actions gomod]", eco)
	}
}

// TestInstallDependabot_NoManifestsIsAnError — emitting a config with no
// update blocks would read as configured while doing nothing.
func TestInstallDependabot_NoManifestsIsAnError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r\n")

	code, _, errOut := runCLI(t, "install-dependabot", "--apply", dir)
	if code != exitCannotRun {
		t.Fatalf("exit = %d, want %d (%s)", code, exitCannotRun, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, dependabot.DefaultPath)); !os.IsNotExist(err) {
		t.Errorf("nothing should have been written")
	}
}

// TestInstallDependabot_RefusesOverwrite — an existing config encodes
// ignores, groups and reviewers that are not ours to replace.
func TestInstallDependabot_RefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n")
	writeFile(t, dir, dependabot.DefaultPath, "version: 2\nupdates: []\n")

	code, _, errOut := runCLI(t, "install-dependabot", "--apply", dir)
	if code != exitCannotRun {
		t.Fatalf("exit = %d, want %d (%s)", code, exitCannotRun, errOut)
	}
	body, _ := os.ReadFile(filepath.Join(dir, dependabot.DefaultPath))
	if string(body) != "version: 2\nupdates: []\n" {
		t.Errorf("existing config was modified: %q", body)
	}
}

func TestInstallDependabot_IntervalAndList(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n")

	code, _, errOut := runCLI(t, "install-dependabot", "--interval", "monthly", "--apply", dir)
	if code != exitOK {
		t.Fatalf("exit = %d (%s)", code, errOut)
	}
	body, _ := os.ReadFile(filepath.Join(dir, dependabot.DefaultPath))
	if !strings.Contains(string(body), "interval: monthly") {
		t.Errorf("--interval ignored:\n%s", body)
	}

	if code, _, _ := runCLI(t, "install-dependabot", "--interval", "hourly", dir); code != exitCannotRun {
		t.Errorf("an interval GitHub rejects should exit %d", exitCannotRun)
	}

	code, out, _ := runCLI(t, "install-dependabot", "--list", dir)
	if code != exitOK || !strings.Contains(out, "gomod") {
		t.Errorf("--list should report gomod:\n%s", out)
	}
}

// TestInstallDependabot_MovesNoVerdict — the scope argument in SPEC.md §4
// rests on this: plumbline must not credit a repo for a file plumbline
// wrote, and nothing in the catalog detects a Dependabot config.
//
// If this test ever fails, do not "fix" it by relaxing the assertion. It
// failing means a signal has started crediting a file plumbline
// generates, which is the circularity §4 exists to prevent.
//
// The durable boundary is what the scaffolder writes, not what the
// catalog happens to miss. In ACMM terms Dependabot alone is an open
// loop — upstream release, PR, a human merges — and the human in the
// path is what keeps it below L4. Auto-merge gated on CI closes it, and
// that closed loop is a real L4 topology (and a no-skip case: safe only
// once the tests deciding it are trustworthy).
//
// install-dependabot deliberately writes only the open half: update
// blocks and a schedule, nothing that merges. A future
// dependency-automation signal should detect the *closed* loop, which
// plumbline does not generate — keeping generator and detector disjoint
// by construction. If you are here because you added such a signal,
// check it is matching auto-merge and not the config file.
func TestInstallDependabot_MovesNoVerdict(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n")
	writeFile(t, dir, "README.md", "# r\n")

	_, before, _ := runCLI(t, "assess", "--report", "json", dir)
	if code, _, errOut := runCLI(t, "install-dependabot", "--apply", dir); code != exitOK {
		t.Fatalf("install-dependabot exit = %d (%s)", code, errOut)
	}
	_, after, _ := runCLI(t, "assess", "--report", "json", dir)

	if before != after {
		t.Errorf("installing a Dependabot config changed the assessment; " +
			"no signal should detect a file plumbline wrote")
	}
}
