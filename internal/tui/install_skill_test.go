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

func TestTUI_IKeyTriggersInstallFlowWhenAbsent(t *testing.T) {
	scan, _ := runScanCount(acmm.Report{
		Repo:    "/test",
		Verdict: acmm.Verdict{Level: acmm.LevelInstructed, Name: "Instructed"},
		Signals: []acmm.SignalResult{{ID: "l2.x", Status: acmm.StatusFound}},
	})
	m := New(scan).(*model)
	completeScan(t, m, m.report)
	m.idx = idxWithFiles(t, fstest.MapFS{"README.md": {Data: []byte("# r")}})

	out, _ := m.Update(keyMsg("i"))
	*m = *(out.(*model))

	if m.screen != screenFixPreview {
		t.Errorf("after 'i' on results, screen = %v, want screenFixPreview", m.screen)
	}
	if m.fixPlan.SignalID != "install-skill" {
		t.Errorf("fixPlan.SignalID = %q, want install-skill", m.fixPlan.SignalID)
	}
	if len(m.fixPlan.Ops) != 1 || m.fixPlan.Ops[0].Path != skill.Path {
		t.Errorf("fixPlan op should target %s; got %+v", skill.Path, m.fixPlan.Ops)
	}
}

func TestTUI_IKeyIsNoOpWhenSkillAlreadyInstalled(t *testing.T) {
	scan, _ := runScanCount(acmm.Report{
		Repo:    "/test",
		Verdict: acmm.Verdict{Level: acmm.LevelInstructed},
		Signals: []acmm.SignalResult{{ID: "l2.x", Status: acmm.StatusFound}},
	})
	m := New(scan).(*model)
	completeScan(t, m, m.report)
	m.idx = idxWithFiles(t, fstest.MapFS{
		skill.Path: {Data: []byte("# already there")},
	})

	out, _ := m.Update(keyMsg("i"))
	*m = *(out.(*model))

	if m.screen != screenResults {
		t.Errorf("'i' should be a no-op when skill is installed; screen = %v", m.screen)
	}
}

func TestTUI_ResultsHintShowsInstallSkillOnlyWhenAbsent(t *testing.T) {
	scan, _ := runScanCount(acmm.Report{
		Repo:    "/test",
		Verdict: acmm.Verdict{Level: acmm.LevelInstructed},
		Signals: []acmm.SignalResult{{ID: "l2.x", Status: acmm.StatusFound}},
	})

	// Skill absent → hint mentions [i] install.
	m := New(scan).(*model)
	completeScan(t, m, m.report)
	m.idx = idxWithFiles(t, fstest.MapFS{"README.md": {Data: []byte("# r")}})
	view := m.View()
	if !strings.Contains(view, "[i] install skill") {
		t.Errorf("results view should advertise [i] install skill when absent; got:\n%s", view)
	}

	// Skill present → hint omits the [i] line.
	m2 := New(scan).(*model)
	completeScan(t, m2, m2.report)
	m2.idx = idxWithFiles(t, fstest.MapFS{
		skill.Path: {Data: []byte("# present")},
	})
	view2 := m2.View()
	if strings.Contains(view2, "[i] install skill") {
		t.Errorf("results view should NOT advertise [i] install skill when present; got:\n%s", view2)
	}
}
