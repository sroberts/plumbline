package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// snapshotInto writes a snapshot of dir to out and fails on error.
func snapshotInto(t *testing.T, dir, out string, extraArgs ...string) {
	t.Helper()
	args := append([]string{"snapshot", "--out", out}, append(extraArgs, dir)...)
	if code, _, errOut := runCLI(t, args...); code != exitOK {
		t.Fatalf("snapshot exit = %d (stderr: %s)", code, errOut)
	}
}

// TestDiff_TextShowsLevelMoveAndSignalDelta drives the whole path #39
// depends on: two committed-style artifacts in, a delta out.
func TestDiff_TextShowsLevelMoveAndSignalDelta(t *testing.T) {
	base := t.TempDir()
	writeFile(t, base, "README.md", "# r\n")

	head := t.TempDir()
	writeFile(t, head, "README.md", "# r\n")
	writeFile(t, head, "CLAUDE.md", "# CLAUDE\n\n"+strings.Repeat("rule.\n", 25))

	baseArt := filepath.Join(t.TempDir(), "base.toon")
	headArt := filepath.Join(t.TempDir(), "head.toon")
	snapshotInto(t, base, baseArt)
	snapshotInto(t, head, headArt)

	code, out, errOut := runCLI(t, "diff", baseArt, headArt)
	if code != exitOK {
		t.Fatalf("diff exit = %d (stderr: %s)", code, errOut)
	}
	if !strings.Contains(out, "l2.agent-instructions`: missing → found") {
		t.Errorf("expected the agent-instructions transition, got:\n%s", out)
	}
	if !strings.Contains(out, "verdict delta") {
		t.Errorf("expected a delta heading, got:\n%s", out)
	}
}

// Formats are inferred per-file from the extension; a .toon base and a
// .json head must diff cleanly against each other.
func TestDiff_MixedFormatsInferredByExtension(t *testing.T) {
	base := t.TempDir()
	writeFile(t, base, "README.md", "# r\n")
	head := t.TempDir()
	writeFile(t, head, "README.md", "# r\n")

	baseArt := filepath.Join(t.TempDir(), "base.toon")
	headArt := filepath.Join(t.TempDir(), "head.json")
	snapshotInto(t, base, baseArt)
	snapshotInto(t, head, headArt, "--format", "json")

	code, out, errOut := runCLI(t, "diff", "--json", baseArt, headArt)
	if code != exitOK {
		t.Fatalf("diff exit = %d (stderr: %s)", code, errOut)
	}
	var delta map[string]any
	if err := json.Unmarshal([]byte(out), &delta); err != nil {
		t.Fatalf("delta is not JSON: %v\n%s", err, out)
	}
	if delta["direction"] != "same" {
		t.Errorf("identical repos should have direction 'same', got %v", delta["direction"])
	}
	if changes, ok := delta["signal_changes"].([]any); ok && len(changes) != 0 {
		t.Errorf("identical repos should have no signal changes, got %v", changes)
	}
}

func TestDiff_MissingFileFailsCannotRun(t *testing.T) {
	base := t.TempDir()
	writeFile(t, base, "README.md", "# r\n")
	baseArt := filepath.Join(t.TempDir(), "base.toon")
	snapshotInto(t, base, baseArt)

	code, _, errOut := runCLI(t, "diff", baseArt, filepath.Join(t.TempDir(), "nope.toon"))
	if code != exitCannotRun {
		t.Fatalf("exit = %d, want %d (stderr: %s)", code, exitCannotRun, errOut)
	}
}

func TestDiff_RoundTripArtifactDecodes(t *testing.T) {
	// A snapshot diffed against itself is a strict no-op — proves the
	// committed artifact decodes back to the same verdict it encodes.
	dir := t.TempDir()
	writeFile(t, dir, "CLAUDE.md", "# CLAUDE\n\n"+strings.Repeat("rule.\n", 25))
	art := filepath.Join(t.TempDir(), "snap.toon")
	snapshotInto(t, dir, art)

	code, out, errOut := runCLI(t, "diff", art, art)
	if code != exitOK {
		t.Fatalf("diff exit = %d (stderr: %s)", code, errOut)
	}
	if !strings.Contains(out, "Verdict unchanged") || !strings.Contains(out, "No signals changed status") {
		t.Errorf("self-diff should be a no-op, got:\n%s", out)
	}
	_ = os.Remove(art)
}
