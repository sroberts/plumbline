package tui

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sroberts/plumbline/internal/ciworkflow"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// ciModel builds a model parked on the results screen with the given
// repo contents.
func ciModel(t *testing.T, files fstest.MapFS) *model {
	t.Helper()
	scan, _ := runScanCount(acmm.Report{
		Repo:    "/test",
		Verdict: acmm.Verdict{Level: acmm.LevelInstructed, Name: "Instructed"},
		Signals: []acmm.SignalResult{{ID: "l2.x", Status: acmm.StatusFound}},
	})
	m := New(scan).(*model)
	completeScan(t, m, m.report)
	m.idx = idxWithFiles(t, files)
	return m
}

// TestTUI_WKeyOpensCIPicker — the TUI must reach the same install-ci
// flow the CLI has (SPEC.md §8.2.1 parity).
func TestTUI_WKeyOpensCIPicker(t *testing.T) {
	m := ciModel(t, fstest.MapFS{"README.md": {Data: []byte("# r")}})

	out, _ := m.Update(keyMsg("w"))
	*m = *(out.(*model))

	if m.screen != screenCIVariants {
		t.Fatalf("after 'w', screen = %v, want screenCIVariants", m.screen)
	}
	if m.ciCursor != 0 {
		t.Errorf("ciCursor = %d, want 0", m.ciCursor)
	}
}

// TestTUI_CIPickerNavigatesAndPreviews walks the picker the way a user
// would and lands on the shared fix-preview screen.
func TestTUI_CIPickerNavigatesAndPreviews(t *testing.T) {
	m := ciModel(t, fstest.MapFS{"README.md": {Data: []byte("# r")}})
	send(m, "w", "down", "enter")

	if m.screen != screenFixPreview {
		t.Fatalf("screen = %v, want screenFixPreview", m.screen)
	}
	if !strings.HasPrefix(m.fixPlan.SignalID, "install-ci:") {
		t.Errorf("plan id = %q, want an install-ci: prefix", m.fixPlan.SignalID)
	}
	// Cursor moved down one row, so the second variant was selected.
	want := ciworkflow.Variants()[1].ID
	if m.fixPlan.SignalID != "install-ci:"+want {
		t.Errorf("plan id = %q, want install-ci:%s", m.fixPlan.SignalID, want)
	}
	if len(m.fixPlan.Ops) != 1 || m.fixPlan.Ops[0].Path != ciworkflow.DefaultPath {
		t.Errorf("plan does not write %s: %+v", ciworkflow.DefaultPath, m.fixPlan.Ops)
	}
}

// TestTUI_CIPickerCyclesGateFloor — the gate floor is the one choice a
// user must make, so the picker has to expose it.
func TestTUI_CIPickerCyclesGateFloor(t *testing.T) {
	m := ciModel(t, fstest.MapFS{"README.md": {Data: []byte("# r")}})
	send(m, "w")

	start := m.ciFailBelow
	send(m, "f")
	if m.ciFailBelow == start {
		t.Fatalf("'f' did not change the gate floor (still %d)", start)
	}

	// Cycling all the way round must return to the starting value and
	// only ever pass through valid floors.
	for i := 0; i < 12; i++ {
		if m.ciFailBelow != 0 && (m.ciFailBelow < 2 || m.ciFailBelow > 5) {
			t.Fatalf("gate floor cycled to invalid value %d", m.ciFailBelow)
		}
		send(m, "f")
	}

	// The chosen floor must reach the rendered workflow.
	for m.ciFailBelow != 3 {
		send(m, "f")
	}
	send(m, "enter")
	if !strings.Contains(string(m.fixPlan.Ops[0].Body), "--fail-below 3") {
		t.Errorf("gate floor not threaded into the workflow:\n%s", m.fixPlan.Ops[0].Body)
	}
}

// TestTUI_CIPickerEscReturnsToResults keeps the picker escapable.
func TestTUI_CIPickerEscReturnsToResults(t *testing.T) {
	m := ciModel(t, fstest.MapFS{"README.md": {Data: []byte("# r")}})
	send(m, "w", "esc")

	if m.screen != screenResults {
		t.Errorf("after esc, screen = %v, want screenResults", m.screen)
	}
}

// TestTUI_CIPreviewCancelReturnsToPicker — cancelling a preview should
// go back where the user came from, not to an unrelated screen.
func TestTUI_CIPreviewCancelReturnsToPicker(t *testing.T) {
	m := ciModel(t, fstest.MapFS{"README.md": {Data: []byte("# r")}})
	send(m, "w", "enter", "esc")

	if m.screen != screenCIVariants {
		t.Errorf("after cancelling a CI preview, screen = %v, want screenCIVariants", m.screen)
	}
}

// TestTUI_CIHintHiddenWhenInstalled — offering to install a workflow
// that is already there is noise, and the create-file op would fail.
func TestTUI_CIHintHiddenWhenInstalled(t *testing.T) {
	absent := ciModel(t, fstest.MapFS{"README.md": {Data: []byte("# r")}})
	if !ciWorkflowMissing(absent.idx) {
		t.Errorf("ciWorkflowMissing = false for a repo with no workflow")
	}
	if !strings.Contains(absent.renderResults(), "install CI") {
		t.Errorf("results footer should offer the CI install:\n%s", absent.renderResults())
	}

	present := ciModel(t, fstest.MapFS{
		ciworkflow.DefaultPath: {Data: []byte("name: mine\n")},
	})
	if ciWorkflowMissing(present.idx) {
		t.Errorf("ciWorkflowMissing = true when the workflow is in the index")
	}
	if strings.Contains(present.renderResults(), "install CI") {
		t.Errorf("results footer should not offer an install that would fail:\n%s", present.renderResults())
	}
}

// TestTUI_CIPickerRenders checks the picker actually shows the choices
// rather than an empty frame.
func TestTUI_CIPickerRenders(t *testing.T) {
	m := ciModel(t, fstest.MapFS{"README.md": {Data: []byte("# r")}})
	send(m, "w")

	out := m.View()
	for _, v := range ciworkflow.Variants() {
		if !strings.Contains(out, v.ID) {
			t.Errorf("picker missing variant %q:\n%s", v.ID, out)
		}
	}
	if !strings.Contains(out, ciworkflow.DefaultPath) {
		t.Errorf("picker does not say where the workflow lands:\n%s", out)
	}
}

// send drives a sequence of keypresses through the model.
func send(m *model, keys ...string) {
	for _, k := range keys {
		out, _ := m.Update(keyMsg(k))
		*m = *(out.(*model))
	}
}
