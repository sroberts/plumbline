package tui

import (
	"context"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// runScanCount returns a ScanFunc that produces the given report and
// counts how many times it has been invoked.
func runScanCount(report acmm.Report) (ScanFunc, *atomic.Int64) {
	var calls atomic.Int64
	scan := func(ctx context.Context) (acmm.Report, *scanner.RepoIndex, error) {
		calls.Add(1)
		return report, &scanner.RepoIndex{Root: "/test"}, nil
	}
	return scan, &calls
}

// keyMsg is a small helper to build a tea.KeyMsg from a string.
func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// completeScan drives the model from screenScanning to screenResults
// by emitting a synthetic doneMsg with the supplied report.
func completeScan(t *testing.T, m *model, report acmm.Report) {
	t.Helper()
	out, _ := m.Update(doneMsg{report: report, idx: &scanner.RepoIndex{Root: "/test"}})
	*m = *(out.(*model))
	if m.screen != screenResults {
		t.Fatalf("after doneMsg, screen = %v, want screenResults", m.screen)
	}
}

func TestTUI_RFromResultsTriggersRescan(t *testing.T) {
	report := acmm.Report{
		Repo:    "/test",
		Verdict: acmm.Verdict{Level: acmm.LevelInstructed, Name: "Instructed"},
		Signals: []acmm.SignalResult{{ID: "l2.x", Status: acmm.StatusFound}},
	}
	scan, _ := runScanCount(report)
	m := New(scan).(*model)

	// Simulate the initial scan completing.
	completeScan(t, m, report)

	// Pressing 'r' on the results screen should switch back to scanning
	// and emit a tea.Cmd that will produce a new doneMsg.
	out, cmd := m.Update(keyMsg("r"))
	*m = *(out.(*model))

	if m.screen != screenScanning {
		t.Errorf("after 'r', screen = %v, want screenScanning", m.screen)
	}
	if cmd == nil {
		t.Fatal("after 'r', expected a tea.Cmd to re-run the scan, got nil")
	}

	// Run the cmd; it should produce a doneMsg with a fresh report.
	msg := cmd()
	if _, ok := msg.(doneMsg); !ok {
		t.Errorf("rescan cmd produced %T, want doneMsg", msg)
	}
}

func TestTUI_RFromFixDoneTriggersRescan(t *testing.T) {
	scan, _ := runScanCount(acmm.Report{Repo: "/test"})
	m := New(scan).(*model)
	m.screen = screenFixDone
	m.fixApplied = true

	out, cmd := m.Update(keyMsg("r"))
	*m = *(out.(*model))

	if m.screen != screenScanning {
		t.Errorf("after 'r' from fix-done, screen = %v, want screenScanning", m.screen)
	}
	if cmd == nil {
		t.Fatal("expected re-scan cmd from fix-done screen")
	}
}

func TestTUI_RIgnoredWhileScanning(t *testing.T) {
	scan, _ := runScanCount(acmm.Report{Repo: "/test"})
	m := New(scan).(*model)
	// Model starts in screenScanning.

	out, _ := m.Update(keyMsg("r"))
	*m = *(out.(*model))

	if m.screen != screenScanning {
		t.Errorf("'r' during scan should be ignored; screen = %v, want screenScanning", m.screen)
	}
}
