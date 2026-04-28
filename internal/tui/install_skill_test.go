package tui

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/skill"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// idxWithFiles builds a RepoIndex from a synthetic file map.
func idxWithFiles(t *testing.T, files fstest.MapFS) *scanner.RepoIndex {
	t.Helper()
	idx, err := scanner.ScanFS(files, "/test")
	if err != nil {
		t.Fatalf("ScanFS: %v", err)
	}
	return idx
}

func TestTUI_SkillInstalledReflectsScanResult(t *testing.T) {
	withSkill := idxWithFiles(t, fstest.MapFS{
		skill.Path: {Data: []byte("# existing skill")},
	})
	withoutSkill := idxWithFiles(t, fstest.MapFS{
		"README.md": {Data: []byte("# r")},
	})

	if !skillIsInstalled(withSkill) {
		t.Error("skillIsInstalled = false when SKILL.md is in the index")
	}
	if skillIsInstalled(withoutSkill) {
		t.Error("skillIsInstalled = true when SKILL.md is NOT in the index")
	}
}

// TestTUI_IKeyOpensTargetPicker — pressing 'i' on results goes to the
// new picker screen (not directly to preview anymore).
func TestTUI_IKeyOpensTargetPicker(t *testing.T) {
	scan, _ := runScanCount(acmm.Report{
		Repo:    "/test",
		Verdict: acmm.Verdict{Level: acmm.LevelInstructed},
		Signals: []acmm.SignalResult{{ID: "l2.x", Status: acmm.StatusFound}},
	})
	m := New(scan).(*model)
	completeScan(t, m, m.report)
	m.idx = idxWithFiles(t, fstest.MapFS{"README.md": {Data: []byte("# r")}})

	out, _ := m.Update(keyMsg("i"))
	*m = *(out.(*model))

	if m.screen != screenSkillTargets {
		t.Errorf("after 'i', screen = %v, want screenSkillTargets", m.screen)
	}
	if m.skillCursor != 0 {
		t.Errorf("skillCursor = %d, want 0", m.skillCursor)
	}
	if m.skillGlobal {
		t.Errorf("skillGlobal = true, want false (default scope)")
	}
}

// TestTUI_PickerEnterBuildsPlanForSelectedTarget — 'enter' on the picker
// generates the FixPlan for the currently-highlighted target.
func TestTUI_PickerEnterBuildsPlanForSelectedTarget(t *testing.T) {
	scan, _ := runScanCount(acmm.Report{Repo: "/test"})
	m := New(scan).(*model)
	completeScan(t, m, m.report)
	m.idx = idxWithFiles(t, fstest.MapFS{"README.md": {Data: []byte("# r")}})

	// Open picker, advance to second target (cursor), pick it.
	m.Update(keyMsg("i"))
	out, _ := m.Update(keyMsg("down"))
	*m = *(out.(*model))
	if m.skillCursor != 1 {
		t.Fatalf("skillCursor after down = %d, want 1", m.skillCursor)
	}
	out, _ = m.Update(keyMsg("enter"))
	*m = *(out.(*model))

	if m.screen != screenFixPreview {
		t.Fatalf("after enter, screen = %v, want screenFixPreview", m.screen)
	}
	want := skill.Targets()[1]
	if !strings.HasSuffix(m.fixPlan.SignalID, want.ID) {
		t.Errorf("fixPlan.SignalID = %q, want suffix %q", m.fixPlan.SignalID, want.ID)
	}
	if len(m.fixPlan.Ops) != 1 || m.fixPlan.Ops[0].Path != want.Path {
		t.Errorf("fixPlan op should target %s; got %+v", want.Path, m.fixPlan.Ops)
	}
}

// TestTUI_PickerGTogglesGlobalScope — 'g' flips skillGlobal.
func TestTUI_PickerGTogglesGlobalScope(t *testing.T) {
	scan, _ := runScanCount(acmm.Report{Repo: "/test"})
	m := New(scan).(*model)
	completeScan(t, m, m.report)
	m.idx = idxWithFiles(t, fstest.MapFS{"README.md": {Data: []byte("# r")}})

	m.Update(keyMsg("i"))
	out, _ := m.Update(keyMsg("g"))
	*m = *(out.(*model))

	if !m.skillGlobal {
		t.Errorf("after 'g', skillGlobal = false, want true")
	}
	out, _ = m.Update(keyMsg("g"))
	*m = *(out.(*model))
	if m.skillGlobal {
		t.Errorf("second 'g' should toggle back to false")
	}
}

// TestTUI_PickerGlobalOnUnsupportedTargetSurfacesError — picking a
// target without a global location while in --global mode goes to
// screenFixDone with a clear error.
func TestTUI_PickerGlobalOnUnsupportedTargetSurfacesError(t *testing.T) {
	scan, _ := runScanCount(acmm.Report{Repo: "/test"})
	m := New(scan).(*model)
	completeScan(t, m, m.report)
	m.idx = idxWithFiles(t, fstest.MapFS{"README.md": {Data: []byte("# r")}})

	m.Update(keyMsg("i"))
	// Move to a target that doesn't support global (windsurf, cline,
	// or copilot — depends on order). Walk down and find one.
	targets := skill.Targets()
	target := -1
	for i, t := range targets {
		if !t.SupportsGlobal() {
			target = i
			break
		}
	}
	if target < 0 {
		t.Skip("no targets without global support")
	}
	for i := 0; i < target; i++ {
		out, _ := m.Update(keyMsg("down"))
		*m = *(out.(*model))
	}
	// Toggle global on, then enter.
	m.Update(keyMsg("g"))
	out, _ := m.Update(keyMsg("enter"))
	*m = *(out.(*model))

	if m.screen != screenFixDone {
		t.Fatalf("expected screenFixDone for unsupported global; screen = %v", m.screen)
	}
	if m.fixErr == nil || !strings.Contains(m.fixErr.Error(), "no documented global location") {
		t.Errorf("expected 'no documented global location' error; got %v", m.fixErr)
	}
}

// TestTUI_ResultsHintShowsInstallSkillWhenAnyTargetMissing — hint is
// hidden only when EVERY supported target is already present.
func TestTUI_ResultsHintShowsInstallSkillWhenAnyTargetMissing(t *testing.T) {
	scan, _ := runScanCount(acmm.Report{
		Repo:    "/test",
		Verdict: acmm.Verdict{Level: acmm.LevelInstructed},
		Signals: []acmm.SignalResult{{ID: "l2.x", Status: acmm.StatusFound}},
	})

	// Some targets present, some not → hint shows.
	m := New(scan).(*model)
	completeScan(t, m, m.report)
	m.idx = idxWithFiles(t, fstest.MapFS{
		skill.Path:  {Data: []byte("# claude here")},
		"README.md": {Data: []byte("# r")},
	})
	view := m.View()
	if !strings.Contains(view, "[i] install skill") {
		t.Errorf("hint should show when at least one target is missing; got:\n%s", view)
	}

	// All targets present → hint hides.
	m2 := New(scan).(*model)
	completeScan(t, m2, m2.report)
	files := fstest.MapFS{}
	for _, t := range skill.Targets() {
		files[t.Path] = &fstest.MapFile{Data: []byte("# present")}
	}
	m2.idx = idxWithFiles(t, files)
	view2 := m2.View()
	if strings.Contains(view2, "[i] install skill") {
		t.Errorf("hint should hide when ALL targets are present; got:\n%s", view2)
	}
}
