package tui

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sroberts/plumbline/internal/dependabot"
)

// TestTUI_DKeyPreviewsDependabotConfig — the TUI must reach the same
// scaffolding the CLI has (SPEC.md §8.2.1 parity).
func TestTUI_DKeyPreviewsDependabotConfig(t *testing.T) {
	m := ciModel(t, fstest.MapFS{
		"README.md":                {Data: []byte("# r")},
		"go.mod":                   {Data: []byte("module x\n")},
		".github/workflows/ci.yml": {Data: []byte("name: CI\non: [push]\njobs: {}\n")},
	})
	send(m, "d")

	if m.screen != screenFixPreview {
		t.Fatalf("after 'd', screen = %v, want screenFixPreview", m.screen)
	}
	if m.fixPlan.SignalID != "install-dependabot" {
		t.Errorf("plan id = %q", m.fixPlan.SignalID)
	}
	if len(m.fixPlan.Ops) != 1 || m.fixPlan.Ops[0].Path != dependabot.DefaultPath {
		t.Fatalf("plan does not write %s: %+v", dependabot.DefaultPath, m.fixPlan.Ops)
	}
	body := string(m.fixPlan.Ops[0].Body)
	for _, want := range []string{"gomod", "github-actions", "version: 2"} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered config missing %q:\n%s", want, body)
		}
	}
}

// TestTUI_DependabotHintHiddenWhenPresent — offering an install that the
// create-file op would refuse is noise.
func TestTUI_DependabotHintHiddenWhenPresent(t *testing.T) {
	absent := ciModel(t, fstest.MapFS{
		"README.md": {Data: []byte("# r")},
		"go.mod":    {Data: []byte("module x\n")},
	})
	if !strings.Contains(absent.renderResults(), "dependabot") {
		t.Errorf("results footer should offer the dependabot install:\n%s", absent.renderResults())
	}

	present := ciModel(t, fstest.MapFS{
		"README.md":            {Data: []byte("# r")},
		"go.mod":               {Data: []byte("module x\n")},
		dependabot.DefaultPath: {Data: []byte("version: 2\nupdates: []\n")},
	})
	if strings.Contains(present.renderResults(), "dependabot") {
		t.Errorf("should not offer an install that would be refused:\n%s", present.renderResults())
	}
}

// TestTUI_DependabotHintHiddenWithNoManifests — nothing to update means
// nothing to offer.
func TestTUI_DependabotHintHiddenWithNoManifests(t *testing.T) {
	m := ciModel(t, fstest.MapFS{"README.md": {Data: []byte("# r")}})
	if strings.Contains(m.renderResults(), "dependabot") {
		t.Errorf("a repo with no manifests has nothing for Dependabot to do:\n%s", m.renderResults())
	}
	// Pressing it anyway must not crash or open an empty preview.
	send(m, "d")
	if m.screen == screenFixPreview {
		t.Errorf("'d' opened a preview for a repo with no manifests")
	}
}

// TestTUI_DependabotPreviewCancelReturnsToResults — the install is
// launched from the results screen, so cancelling belongs there, not on
// the detail screen for whatever signal happened to be selected.
func TestTUI_DependabotPreviewCancelReturnsToResults(t *testing.T) {
	m := ciModel(t, fstest.MapFS{
		"README.md": {Data: []byte("# r")},
		"go.mod":    {Data: []byte("module x\n")},
	})
	send(m, "d", "esc")
	if m.screen != screenResults {
		t.Errorf("after cancelling a dependabot preview, screen = %v, want screenResults", m.screen)
	}
}
