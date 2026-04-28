package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSkill_DryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	code, out, _ := runCLI(t, "install-skill", dir)
	if code != exitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "DRY-RUN") {
		t.Errorf("dry-run output should say so; got:\n%s", out)
	}
	target := filepath.Join(dir, ".claude", "skills", "plumbline", "SKILL.md")
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("dry-run must not create the skill file at %s", target)
	}
}

func TestInstallSkill_ApplyWritesSkill(t *testing.T) {
	dir := t.TempDir()
	code, _, errOut := runCLI(t, "install-skill", dir, "--apply")
	if code != exitOK {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	target := filepath.Join(dir, ".claude", "skills", "plumbline", "SKILL.md")
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("expected skill at %s, got: %v", target, err)
	}
	body := string(got)
	for _, want := range []string{
		"name: plumbline",
		"description:",
		"plumbline --json",
		"plumbline fix",
		"plumbline inspect",
		"l2.agent-instructions",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("skill body missing %q", want)
		}
	}
}

func TestInstallSkill_RefusesToOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".claude", "skills", "plumbline", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("user-customized skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, _, errOut := runCLI(t, "install-skill", dir, "--apply")
	if code == exitOK {
		t.Errorf("expected non-zero exit when skill already exists")
	}
	if !strings.Contains(errOut, "already exists") {
		t.Errorf("error message should mention 'already exists'; got: %s", errOut)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "user-customized skill" {
		t.Errorf("existing skill was overwritten; content=%q", got)
	}
}

func TestInstallSkill_RootCommandIsRegistered(t *testing.T) {
	code, out, _ := runCLI(t, "install-skill", "--help")
	if code != exitOK {
		t.Fatalf("exit = %d, want 0 (--help should always succeed)", code)
	}
	if !strings.Contains(out, "Claude Code skill") {
		t.Errorf("--help output should mention Claude Code skill; got:\n%s", out)
	}
}
