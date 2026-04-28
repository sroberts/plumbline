// E2E tests for `plumbline install-skill`. These build the actual
// binary and exec it as a subprocess, so they catch issues the
// in-process tests miss: ldflag injection, exit-code propagation
// across the OS boundary, $HOME resolution, real file writes, real
// stderr/stdout streams.
//
// Each test invocation rebuilds the binary if the cache is cold. The
// build is cached across tests in a TestMain-managed temp dir.
package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	e2eBuildOnce sync.Once
	e2eBinPath   string
	e2eBuildErr  error
)

// e2eBuild compiles the plumbline binary into a temp dir on first
// use; subsequent calls return the cached path. Returns an empty
// string + error if the build failed (the test should t.Fatal).
func e2eBuild(t *testing.T) string {
	t.Helper()
	e2eBuildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "plumbline-e2e-*")
		if err != nil {
			e2eBuildErr = err
			return
		}
		bin := filepath.Join(dir, "plumbline")
		cmd := exec.Command("go", "build", "-o", bin, ".")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			e2eBuildErr = err
			return
		}
		e2eBinPath = bin
	})
	if e2eBuildErr != nil {
		t.Fatalf("e2e build failed: %v", e2eBuildErr)
	}
	return e2eBinPath
}

// runBin execs the compiled binary with args, returning exit code,
// stdout, and stderr. extraEnv is appended to os.Environ() (so
// callers can sandbox HOME, etc.).
func runBin(t *testing.T, extraEnv []string, args ...string) (int, string, string) {
	t.Helper()
	bin := e2eBuild(t)
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("unexpected exec error: %v", err)
	}
	return code, stdout.String(), stderr.String()
}

func TestE2E_InstallSkill_ListEnumeratesAllTargets(t *testing.T) {
	code, out, _ := runBin(t, nil, "install-skill", "--list")
	if code != 0 {
		t.Fatalf("--list exit = %d, want 0", code)
	}
	for _, want := range []string{
		"claude", "cursor", "gemini", "codex", "opencode",
		"windsurf", "cline", "copilot",
		".claude/skills/plumbline/SKILL.md",
		".cursor/rules/plumbline.mdc",
		"GEMINI.md", "AGENTS.md", ".windsurfrules",
		".clinerules", ".github/copilot-instructions.md",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("--list output missing %q\nFull output:\n%s", want, out)
		}
	}
}

func TestE2E_InstallSkill_ProjectScopeWritesExpectedFile(t *testing.T) {
	cases := []struct {
		target   string
		wantPath string
	}{
		{"claude", ".claude/skills/plumbline/SKILL.md"},
		{"cursor", ".cursor/rules/plumbline.mdc"},
		{"gemini", "GEMINI.md"},
		{"codex", "AGENTS.md"},
		{"opencode", "AGENTS.md"},
		{"windsurf", ".windsurfrules"},
		{"cline", ".clinerules"},
		{"copilot", ".github/copilot-instructions.md"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.target, func(t *testing.T) {
			repo := t.TempDir()
			code, _, errOut := runBin(t, nil,
				"install-skill", repo, "--target", c.target, "--apply")
			if code != 0 {
				t.Fatalf("install-skill --target %s exit = %d (stderr: %s)",
					c.target, code, errOut)
			}
			data, err := os.ReadFile(filepath.Join(repo, c.wantPath))
			if err != nil {
				t.Fatalf("expected %s in %s, got: %v", c.wantPath, repo, err)
			}
			if !strings.Contains(string(data), "plumbline workflow") {
				t.Errorf("file at %s missing core plumbline guide", c.wantPath)
			}
		})
	}
}

func TestE2E_InstallSkill_GlobalScopeUsesSandboxedHome(t *testing.T) {
	home := t.TempDir()
	cases := []struct {
		target   string
		wantPath string
	}{
		{"claude", ".claude/skills/plumbline/SKILL.md"},
		{"cursor", ".cursor/rules/plumbline.mdc"},
		{"gemini", ".gemini/GEMINI.md"},
		{"codex", ".codex/AGENTS.md"},
		{"opencode", ".config/opencode/AGENTS.md"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.target, func(t *testing.T) {
			// Each global install uses its own home so we don't trip
			// the no-overwrite check between subtests.
			h := filepath.Join(home, c.target)
			if err := os.MkdirAll(h, 0o755); err != nil {
				t.Fatal(err)
			}
			code, _, errOut := runBin(t, []string{"HOME=" + h},
				"install-skill", "--target", c.target, "--global", "--apply")
			if code != 0 {
				t.Fatalf("global install --target %s exit = %d (stderr: %s)",
					c.target, code, errOut)
			}
			full := filepath.Join(h, c.wantPath)
			if _, err := os.Stat(full); err != nil {
				t.Errorf("expected file at %s, got: %v", full, err)
			}
		})
	}
}

func TestE2E_InstallSkill_GlobalRejectedForUnsupportedTarget(t *testing.T) {
	home := t.TempDir()
	for _, target := range []string{"windsurf", "cline", "copilot"} {
		target := target
		t.Run(target, func(t *testing.T) {
			code, _, errOut := runBin(t, []string{"HOME=" + home},
				"install-skill", "--target", target, "--global")
			if code == 0 {
				t.Errorf("--global on %s should fail; got exit 0", target)
			}
			if !strings.Contains(errOut, "no documented global location") {
				t.Errorf("error should explain why %s has no global location; got: %s", target, errOut)
			}
		})
	}
}

func TestE2E_InstallSkill_RefusesToOverwrite(t *testing.T) {
	repo := t.TempDir()
	target := filepath.Join(repo, ".claude", "skills", "plumbline", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("user customization"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, _, errOut := runBin(t, nil,
		"install-skill", repo, "--target", "claude", "--apply")
	if code == 0 {
		t.Errorf("re-install should fail; got exit 0")
	}
	if !strings.Contains(errOut, "already exists") {
		t.Errorf("error should mention 'already exists'; got: %s", errOut)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "user customization" {
		t.Errorf("existing file was clobbered; content = %q", got)
	}
}

func TestE2E_InstallSkill_DryRunDoesNotWrite(t *testing.T) {
	repo := t.TempDir()
	code, out, _ := runBin(t, nil, "install-skill", repo, "--target", "gemini")
	if code != 0 {
		t.Fatalf("dry-run exit = %d, want 0", code)
	}
	if !strings.Contains(out, "DRY-RUN") {
		t.Errorf("dry-run output should say DRY-RUN; got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(repo, "GEMINI.md")); err == nil {
		t.Errorf("dry-run wrote GEMINI.md anyway")
	}
}

func TestE2E_InstallSkill_GlobalRejectsPositionalPath(t *testing.T) {
	home := t.TempDir()
	code, _, errOut := runBin(t, []string{"HOME=" + home},
		"install-skill", "/some/repo/path", "--target", "claude", "--global", "--apply")
	if code == 0 {
		t.Error("--global with positional path should fail")
	}
	if !strings.Contains(errOut, "do not pass [path]") {
		t.Errorf("error should clarify; got: %s", errOut)
	}
	// Make sure nothing was written to either path.
	if _, err := os.Stat(filepath.Join(home, ".claude")); err == nil {
		t.Error("aborted install still wrote to HOME")
	}
}

func TestE2E_InstallSkill_UnknownTargetExitsCannotRun(t *testing.T) {
	repo := t.TempDir()
	code, _, errOut := runBin(t, nil,
		"install-skill", repo, "--target", "definitely-not-a-tool")
	if code == 0 {
		t.Error("unknown target should fail")
	}
	if code != exitCannotRun {
		t.Errorf("unknown target exit = %d, want %d (cannot-run)", code, exitCannotRun)
	}
	if !strings.Contains(errOut, "unknown install target") {
		t.Errorf("error should mention unknown target; got: %s", errOut)
	}
}

func TestE2E_InstallSkill_SharedFileWarningOnStderr(t *testing.T) {
	repo := t.TempDir()
	_, _, errOut := runBin(t, nil,
		"install-skill", repo, "--target", "codex", "--apply")
	if !strings.Contains(errOut, "AGENTS.md") || !strings.Contains(errOut, "agent tools") {
		t.Errorf("codex install should warn that AGENTS.md is shared; stderr: %s", errOut)
	}
}
