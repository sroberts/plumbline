// Tests that footer hint lines wrap to the current viewport width
// instead of overflowing it. Closes issue #19. The previous bug was
// that long keybinding strings like "[↑/↓] select   [enter] preview
// [g] scope   [esc] back   [q] quit" overran narrow terminals,
// pushing later content off-screen or wrapping mid-glyph.
//
// Tests intentionally check only footer-hint lines, not the full
// rendered view: signal rows / level bars / target paths can be longer
// than the viewport in narrow terminals (and will be addressed
// separately). Footer hint overflow is the regression that motivated
// the issue.
package tui

import (
	"strings"
	"testing"
	"testing/fstest"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// footerMarkers identifies a rendered line as a footer hint by
// looking for keybinding bracket syntax that styleHint lines use.
var footerMarkers = []string{"[r]", "[g]", "[↑/↓]", "[esc", "[enter]", "[q]", "[y]", "[n/esc]"}

// overflowingFooterLines returns rendered footer-hint lines that
// exceed width (rune-aware, ANSI-stripped).
func overflowingFooterLines(view string, width int) []string {
	var bad []string
	for _, line := range strings.Split(view, "\n") {
		visible := ansi.Strip(line)
		isFooter := false
		for _, m := range footerMarkers {
			if strings.Contains(visible, m) {
				isFooter = true
				break
			}
		}
		if !isFooter {
			continue
		}
		if utf8.RuneCountInString(visible) > width {
			bad = append(bad, visible)
		}
	}
	return bad
}

func newResultsModel(t *testing.T, width int) *model {
	t.Helper()
	idx, err := scanner.ScanFS(fstest.MapFS{
		"README.md": &fstest.MapFile{Data: []byte("# r")},
	}, t.TempDir())
	if err != nil {
		t.Fatalf("ScanFS: %v", err)
	}
	m := New(nil).(*model)
	m.width = width
	m.height = 30
	m.screen = screenResults
	m.report = acmm.Report{
		Repo:    "/test",
		Verdict: acmm.Verdict{Level: acmm.LevelInstructed, Name: "Instructed"},
		Signals: []acmm.SignalResult{
			{ID: "l2.agent-instructions", Status: acmm.StatusFound, Title: "Agent instructions"},
			{ID: "l3.user-feedback", Status: acmm.StatusMissing, Title: "User feedback"},
		},
	}
	m.idx = idx
	return m
}

func TestFooterWrap_ResultsScreenAtNarrowWidth(t *testing.T) {
	for _, w := range []int{40, 60, 80, 120} {
		m := newResultsModel(t, w)
		bad := overflowingFooterLines(m.View(), w)
		if len(bad) > 0 {
			t.Errorf("results @ width=%d: %d footer line(s) exceed width:\n  %s",
				w, len(bad), strings.Join(bad, "\n  "))
		}
	}
}

func TestFooterWrap_DetailScreenAtNarrowWidth(t *testing.T) {
	for _, w := range []int{40, 60, 80, 120} {
		m := newResultsModel(t, w)
		m.screen = screenDetail
		m.cursor = 0
		bad := overflowingFooterLines(m.View(), w)
		if len(bad) > 0 {
			t.Errorf("detail @ width=%d: %d footer line(s) exceed width:\n  %s",
				w, len(bad), strings.Join(bad, "\n  "))
		}
	}
}

func TestFooterWrap_FixDoneScreenAtNarrowWidth(t *testing.T) {
	for _, w := range []int{40, 60, 80, 120} {
		m := newResultsModel(t, w)
		m.screen = screenFixDone
		m.fixApplied = false
		bad := overflowingFooterLines(m.View(), w)
		if len(bad) > 0 {
			t.Errorf("fix-done @ width=%d: %d footer line(s) exceed width:\n  %s",
				w, len(bad), strings.Join(bad, "\n  "))
		}
	}
}

func TestFooterWrap_SkillTargetsScreenAtNarrowWidth(t *testing.T) {
	// SkillTargets footer is the longest in the TUI:
	//   "[↑/↓] select   [enter] preview   [g] scope   [esc] back   [q] quit"
	// — the regression that motivated #19.
	for _, w := range []int{40, 60, 80, 120} {
		m := newResultsModel(t, w)
		m.screen = screenSkillTargets
		bad := overflowingFooterLines(m.View(), w)
		if len(bad) > 0 {
			t.Errorf("skill-targets @ width=%d: %d footer line(s) exceed width:\n  %s",
				w, len(bad), strings.Join(bad, "\n  "))
		}
	}
}

func TestRenderHint_WrapsAtViewWidth(t *testing.T) {
	m := &model{width: 30}
	hint := "[↑/↓] select   [enter] preview   [g] scope   [esc] back   [q] quit"
	got := m.renderHint(hint)
	for _, line := range strings.Split(got, "\n") {
		visible := ansi.Strip(line)
		if utf8.RuneCountInString(visible) > 30 {
			t.Errorf("renderHint produced line %q (%d cols) > 30",
				visible, utf8.RuneCountInString(visible))
		}
	}
	// Should produce at least 2 lines for a 67-char hint at width 30.
	if lines := strings.Split(got, "\n"); len(lines) < 2 {
		t.Errorf("expected wrap into multiple lines, got %d", len(lines))
	}
}

func TestRenderHint_ZeroWidthFallsBackTo80(t *testing.T) {
	m := &model{width: 0}
	hint := strings.Repeat("a ", 50) // 100 chars
	got := m.renderHint(hint)
	// At fallback width 80, "a " repeated 50 times wraps to 2 lines.
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Errorf("expected wrap at fallback width 80, got %d line(s)", len(lines))
	}
}
