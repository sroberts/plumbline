package fix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sroberts/plumbline/pkg/acmm"
)

func TestApply_DryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	plan := acmm.FixPlan{
		SignalID: "l2.test",
		Ops: []acmm.FixOp{{
			Kind: acmm.FixCreateFile,
			Path: "CLAUDE.md",
			Body: []byte("# CLAUDE.md\n\nrules\n"),
		}},
	}

	res, err := Apply(dir, plan, Options{DryRun: true})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(res.Operations) != 1 {
		t.Errorf("expected 1 op result, got %d", len(res.Operations))
	}
	if res.Operations[0].Wrote {
		t.Errorf("DryRun: Wrote=true, want false")
	}
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Errorf("DryRun must not write files, but CLAUDE.md exists: err=%v", err)
	}
}

func TestApply_WriteCreatesFile(t *testing.T) {
	dir := t.TempDir()
	plan := acmm.FixPlan{
		SignalID: "l2.test",
		Ops: []acmm.FixOp{{
			Kind: acmm.FixCreateFile,
			Path: "CLAUDE.md",
			Body: []byte("# CLAUDE.md\n\nrules\n"),
		}},
	}

	res, err := Apply(dir, plan, Options{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !res.Operations[0].Wrote {
		t.Errorf("expected Wrote=true after non-dry-run")
	}
	got, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if string(got) != "# CLAUDE.md\n\nrules\n" {
		t.Errorf("file content = %q, want the body", got)
	}
}

func TestApply_RefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(existing, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan := acmm.FixPlan{
		Ops: []acmm.FixOp{{
			Kind: acmm.FixCreateFile,
			Path: "CLAUDE.md",
			Body: []byte("new"),
		}},
	}

	_, err := Apply(dir, plan, Options{})
	if err == nil {
		t.Fatal("expected error when CreateFile target already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error doesn't mention 'already exists': %v", err)
	}
	got, _ := os.ReadFile(existing)
	if string(got) != "existing" {
		t.Errorf("file was overwritten despite refusal: %q", got)
	}
}

func TestApply_RejectsPathOutsideRoot(t *testing.T) {
	dir := t.TempDir()
	plan := acmm.FixPlan{
		Ops: []acmm.FixOp{{
			Kind: acmm.FixCreateFile,
			Path: "../escape.md",
			Body: []byte("nope"),
		}},
	}
	_, err := Apply(dir, plan, Options{})
	if err == nil {
		t.Fatal("expected error for ../escape.md path")
	}
	if !strings.Contains(err.Error(), "outside") && !strings.Contains(err.Error(), "escape") && !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error doesn't reflect the safety rule: %v", err)
	}
}

func TestApply_RejectsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	plan := acmm.FixPlan{
		Ops: []acmm.FixOp{{
			Kind: acmm.FixCreateFile,
			Path: "/tmp/anywhere.md",
			Body: []byte("nope"),
		}},
	}
	_, err := Apply(dir, plan, Options{})
	if err == nil {
		t.Fatal("expected error for absolute path /tmp/anywhere.md")
	}
}

func TestApply_AppendRequiresExistingFile(t *testing.T) {
	dir := t.TempDir()
	plan := acmm.FixPlan{
		Ops: []acmm.FixOp{{
			Kind: acmm.FixAppendFile,
			Path: "missing.md",
			Body: []byte("more"),
		}},
	}
	_, err := Apply(dir, plan, Options{})
	if err == nil {
		t.Fatal("AppendFile to missing path should error")
	}
}

func TestApply_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(target, []byte("# head\n\nfirst.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan := acmm.FixPlan{
		Ops: []acmm.FixOp{{
			Kind: acmm.FixAppendFile,
			Path: "CLAUDE.md",
			Body: []byte("more guidance.\n"),
		}},
	}
	if _, err := Apply(dir, plan, Options{}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got, _ := os.ReadFile(target)
	if !strings.Contains(string(got), "first.") || !strings.Contains(string(got), "more guidance.") {
		t.Errorf("expected both old and new content; got:\n%s", got)
	}
}

func TestApply_RejectsUnknownOp(t *testing.T) {
	dir := t.TempDir()
	plan := acmm.FixPlan{
		Ops: []acmm.FixOp{{
			Kind: acmm.FixOpKind("delete-everything"),
			Path: "CLAUDE.md",
		}},
	}
	_, err := Apply(dir, plan, Options{})
	if err == nil {
		t.Fatal("expected error for unknown FixOpKind")
	}
}
