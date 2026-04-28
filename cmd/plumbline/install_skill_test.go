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
	for _, want := range []string{"Claude Code", "Cursor", "Codex", "OpenCode", "--target", "--list"} {
		if !strings.Contains(out, want) {
			t.Errorf("--help output should mention %q; got:\n%s", want, out)
		}
	}
}

func TestInstallSkill_ListPrintsAllTargets(t *testing.T) {
	code, out, _ := runCLI(t, "install-skill", "--list")
	if code != exitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{
		"claude", "cursor", "codex", "opencode", "windsurf", "cline", "copilot",
		".claude/skills/plumbline/SKILL.md",
		".cursor/rules/plumbline.mdc",
		"AGENTS.md",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("--list output missing %q; got:\n%s", want, out)
		}
	}
}

func TestInstallSkill_TargetCursorWritesMDC(t *testing.T) {
	dir := t.TempDir()
	code, _, errOut := runCLI(t, "install-skill", dir, "--target", "cursor", "--apply")
	if code != exitOK {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	target := filepath.Join(dir, ".cursor", "rules", "plumbline.mdc")
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("expected file at %s, got: %v", target, err)
	}
	body := string(got)
	if !strings.HasPrefix(body, "---") {
		t.Errorf("Cursor MDC body should start with frontmatter; got prefix %q", body[:min(40, len(body))])
	}
	for _, want := range []string{"alwaysApply", "globs:", "plumbline workflow"} {
		if !strings.Contains(body, want) {
			t.Errorf("cursor body missing %q", want)
		}
	}
}

func TestInstallSkill_TargetCodexWritesAgentsMD(t *testing.T) {
	dir := t.TempDir()
	code, _, errOut := runCLI(t, "install-skill", dir, "--target", "codex", "--apply")
	if code != exitOK {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	if !strings.Contains(errOut, "AGENTS.md") || !strings.Contains(errOut, "agent tools") {
		t.Errorf("codex install should emit a shared-file note on stderr; got: %s", errOut)
	}
	target := filepath.Join(dir, "AGENTS.md")
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("expected AGENTS.md at %s, got: %v", target, err)
	}
	if !strings.Contains(string(got), "plumbline workflow") {
		t.Errorf("AGENTS.md should contain core plumbline guide; got prefix:\n%s", string(got)[:min(200, len(got))])
	}
}

func TestInstallSkill_TargetOpenCodeAlsoWritesAgentsMD(t *testing.T) {
	dir := t.TempDir()
	code, _, _ := runCLI(t, "install-skill", dir, "--target", "opencode", "--apply")
	if code != exitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Errorf("opencode target should also write AGENTS.md")
	}
}

func TestInstallSkill_TargetGeminiWritesGEMINIMD(t *testing.T) {
	dir := t.TempDir()
	code, _, _ := runCLI(t, "install-skill", dir, "--target", "gemini", "--apply")
	if code != exitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	got, err := os.ReadFile(filepath.Join(dir, "GEMINI.md"))
	if err != nil {
		t.Fatalf("expected GEMINI.md, got: %v", err)
	}
	if !strings.Contains(string(got), "Gemini") {
		t.Errorf("GEMINI.md should mention Gemini in its preamble; got prefix:\n%s", string(got)[:min(200, len(got))])
	}
	if !strings.Contains(string(got), "plumbline workflow") {
		t.Errorf("GEMINI.md should contain the core plumbline guide")
	}
}

func TestInstallSkill_GlobalRespectsUserHome(t *testing.T) {
	// Sandbox $HOME so the test doesn't pollute the real user's home dir.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	code, _, errOut := runCLI(t, "install-skill", "--target", "claude", "--global", "--apply")
	if code != exitOK {
		t.Fatalf("--global install exit = %d, want 0 (stderr: %s)", code, errOut)
	}
	got, err := os.ReadFile(filepath.Join(tmpHome, ".claude", "skills", "plumbline", "SKILL.md"))
	if err != nil {
		t.Fatalf("global install should land at $HOME/.claude/skills/plumbline/SKILL.md, got: %v", err)
	}
	if !strings.Contains(string(got), "plumbline workflow") {
		t.Error("global skill body missing the guide")
	}
}

func TestInstallSkill_GlobalRejectsTargetWithoutGlobalLocation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// .windsurfrules has no documented global location.
	code, _, errOut := runCLI(t, "install-skill", "--target", "windsurf", "--global")
	if code == exitOK {
		t.Errorf("expected non-zero exit for --global on windsurf")
	}
	if !strings.Contains(errOut, "no documented global location") {
		t.Errorf("error should explain why; got: %s", errOut)
	}
}

func TestInstallSkill_GlobalRejectsPositionalPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	code, _, errOut := runCLI(t, "install-skill", "/some/repo", "--target", "claude", "--global")
	if code == exitOK {
		t.Errorf("expected non-zero exit when --global is paired with a path arg")
	}
	if !strings.Contains(errOut, "do not pass [path]") {
		t.Errorf("error should clarify --global vs path; got: %s", errOut)
	}
}

func TestInstallSkill_UnknownTargetErrors(t *testing.T) {
	dir := t.TempDir()
	code, _, errOut := runCLI(t, "install-skill", dir, "--target", "definitely-not-a-tool")
	if code == exitOK {
		t.Error("expected non-zero exit for unknown target")
	}
	if !strings.Contains(errOut, "unknown install target") {
		t.Errorf("error should mention unknown target; got: %s", errOut)
	}
}
