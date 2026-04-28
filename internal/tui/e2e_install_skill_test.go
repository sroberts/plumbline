// E2E tests for the TUI install-skill flow. teatest drives the actual
// Bubble Tea program through Init / Update / View loops, so these
// catch issues that the model-level unit tests miss: screen rendering,
// asynchronous tea.Cmd execution, and integration with fix.Apply
// against a real filesystem.
package tui

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/skill"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// e2eHarness wires a real RepoIndex (from a temp dir) into a teatest
// program so the install path actually writes files. Returns the test
// model + the absolute repo path the model was scanned from.
func e2eHarness(t *testing.T) (*teatest.TestModel, string) {
	t.Helper()
	repo := t.TempDir()

	scan := func(ctx context.Context) (acmm.Report, *scanner.RepoIndex, error) {
		idx, err := scanner.ScanFS(fstest.MapFS{
			"README.md": &fstest.MapFile{Data: []byte("# r")},
		}, repo)
		if err != nil {
			return acmm.Report{}, nil, err
		}
		return acmm.Report{
			Repo:    repo, // fix.Apply target — must be absolute
			Verdict: acmm.Verdict{Level: acmm.LevelInstructed, Name: "Instructed"},
			Signals: []acmm.SignalResult{{ID: "l2.x", Status: acmm.StatusFound}},
		}, idx, nil
	}

	tm := teatest.NewTestModel(t, New(scan), teatest.WithInitialTermSize(120, 40))
	return tm, repo
}

// waitForOutput waits until the rendered output matches re, or fails.
func waitForOutput(t *testing.T, tm *teatest.TestModel, re *regexp.Regexp) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return re.Match(b)
	}, teatest.WithDuration(3*time.Second), teatest.WithCheckInterval(20*time.Millisecond))
}

func sendKey(tm *teatest.TestModel, s string) {
	switch s {
	case "enter":
		tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	case "esc":
		tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	case "down":
		tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	case "up":
		tm.Send(tea.KeyMsg{Type: tea.KeyUp})
	default:
		for _, r := range s {
			tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
	}
}

func TestE2E_TUI_InstallClaudeSkillEndToEnd(t *testing.T) {
	tm, repo := e2eHarness(t)

	// Wait for the results screen.
	waitForOutput(t, tm, regexp.MustCompile(`Assessed level`))

	// Open picker.
	sendKey(tm, "i")
	waitForOutput(t, tm, regexp.MustCompile(`Install plumbline skill`))

	// Default cursor is on claude (first target). Enter → preview.
	sendKey(tm, "enter")
	waitForOutput(t, tm, regexp.MustCompile(`(?s)Preview.*plumbline workflow`))

	// Confirm install.
	sendKey(tm, "y")
	waitForOutput(t, tm, regexp.MustCompile(`Applied`))

	// Quit cleanly.
	sendKey(tm, "q")
	if err := tm.Quit(); err != nil {
		// Quit may already have happened from the q key; ignore harmless errors.
		t.Logf("Quit: %v", err)
	}
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	// Verify the file actually got written by fix.Apply.
	got, err := os.ReadFile(filepath.Join(repo, ".claude", "skills", "plumbline", "SKILL.md"))
	if err != nil {
		t.Fatalf("expected SKILL.md to exist, got: %v", err)
	}
	if !strings.Contains(string(got), "name: plumbline") {
		t.Error("SKILL.md missing frontmatter")
	}
}

func TestE2E_TUI_InstallCursorViaPickerNavigation(t *testing.T) {
	tm, repo := e2eHarness(t)
	waitForOutput(t, tm, regexp.MustCompile(`Assessed level`))

	// Open picker, advance once (claude → cursor at index 1), confirm.
	sendKey(tm, "i")
	waitForOutput(t, tm, regexp.MustCompile(`Install plumbline skill`))
	sendKey(tm, "down")
	sendKey(tm, "enter")
	waitForOutput(t, tm, regexp.MustCompile(`Preview`))
	sendKey(tm, "y")
	waitForOutput(t, tm, regexp.MustCompile(`Applied`))

	sendKey(tm, "q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	got, err := os.ReadFile(filepath.Join(repo, ".cursor", "rules", "plumbline.mdc"))
	if err != nil {
		t.Fatalf("expected plumbline.mdc, got: %v", err)
	}
	if !strings.Contains(string(got), "alwaysApply") {
		t.Error("Cursor MDC missing rule frontmatter")
	}
}

func TestE2E_TUI_GTogglesGlobalScopeAndPicksGemini(t *testing.T) {
	// Sandbox HOME so the global install lands somewhere we can inspect.
	home := t.TempDir()
	t.Setenv("HOME", home)
	tm, _ := e2eHarness(t)
	waitForOutput(t, tm, regexp.MustCompile(`Assessed level`))

	// Open picker, toggle global, advance to gemini (index 2 in the
	// canonical order: claude / cursor / gemini), confirm.
	sendKey(tm, "i")
	waitForOutput(t, tm, regexp.MustCompile(`Install plumbline skill`))
	sendKey(tm, "g")
	waitForOutput(t, tm, regexp.MustCompile(`Scope:.*user`))

	// Move down twice (claude → cursor → gemini).
	sendKey(tm, "down")
	sendKey(tm, "down")
	sendKey(tm, "enter")
	waitForOutput(t, tm, regexp.MustCompile(`Preview`))
	sendKey(tm, "y")
	waitForOutput(t, tm, regexp.MustCompile(`Applied`))

	sendKey(tm, "q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	got, err := os.ReadFile(filepath.Join(home, ".gemini", "GEMINI.md"))
	if err != nil {
		t.Fatalf("expected ~/.gemini/GEMINI.md after global gemini install, got: %v", err)
	}
	if !strings.Contains(string(got), "plumbline workflow") {
		t.Error("GEMINI.md missing core guide")
	}
}

func TestE2E_TUI_GlobalOnUnsupportedTargetSurfacesError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tm, _ := e2eHarness(t)
	waitForOutput(t, tm, regexp.MustCompile(`Assessed level`))

	// Open picker, toggle global, walk to an unsupported target.
	sendKey(tm, "i")
	waitForOutput(t, tm, regexp.MustCompile(`Install plumbline skill`))
	sendKey(tm, "g")

	// Find first target without global support and walk down to it.
	target := -1
	for i, t := range skill.Targets() {
		if !t.SupportsGlobal() {
			target = i
			break
		}
	}
	if target < 0 {
		t.Skip("all targets support global; nothing to test")
	}
	for i := 0; i < target; i++ {
		sendKey(tm, "down")
	}
	sendKey(tm, "enter")

	// Should land on screenFixDone with an error mentioning global.
	waitForOutput(t, tm, regexp.MustCompile(`(?i)Failed.*no documented global location`))

	sendKey(tm, "q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestE2E_TUI_EscFromPickerReturnsToResults(t *testing.T) {
	tm, _ := e2eHarness(t)
	waitForOutput(t, tm, regexp.MustCompile(`Assessed level`))

	sendKey(tm, "i")
	waitForOutput(t, tm, regexp.MustCompile(`Install plumbline skill`))
	sendKey(tm, "esc")
	waitForOutput(t, tm, regexp.MustCompile(`Assessed level`))

	sendKey(tm, "q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestE2E_TUI_CancelPreviewWithN(t *testing.T) {
	tm, repo := e2eHarness(t)
	waitForOutput(t, tm, regexp.MustCompile(`Assessed level`))

	sendKey(tm, "i")
	waitForOutput(t, tm, regexp.MustCompile(`Install plumbline skill`))
	sendKey(tm, "enter")
	waitForOutput(t, tm, regexp.MustCompile(`Preview`))

	// Cancel; should NOT write the file.
	sendKey(tm, "n")

	sendKey(tm, "q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	if _, err := os.Stat(filepath.Join(repo, ".claude", "skills", "plumbline", "SKILL.md")); err == nil {
		t.Error("cancelled install still wrote SKILL.md")
	}
}
