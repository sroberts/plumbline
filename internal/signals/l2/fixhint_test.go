package l2

import (
	"strings"
	"testing"
	"testing/fstest"
)

// TestClaudeMD_PartialPopulatesFixAndNotes — partial verdicts must
// surface (a) why they're partial in Notes and (b) a fix recipe in
// FixHint, so the TUI / inspect / markdown can show "here's what's
// missing and here's how to fix it."
func TestClaudeMD_PartialPopulatesFixAndNotes(t *testing.T) {
	// 5 lines + heading = incomplete (heading present, body too short).
	files := fstest.MapFS{
		"CLAUDE.md": {Data: []byte("# CLAUDE.md\n\nshort.\n")},
	}
	got := runSignal(t, ClaudeMD{}, files)
	if got.FixHint == "" {
		t.Errorf("partial result has empty FixHint; need a recipe to display")
	}
	if len(got.Notes) == 0 {
		t.Errorf("partial result has empty Notes; need a 'why' explanation")
	}
	// The Notes should mention the line count or threshold.
	joined := strings.Join(got.Notes, " ")
	if !strings.Contains(joined, "30") && !strings.Contains(joined, "non-blank") {
		t.Errorf("Notes do not explain why this is partial; got: %v", got.Notes)
	}
}

func TestCopilotInstructions_PartialPopulatesFixAndNotes(t *testing.T) {
	// Heading present, fewer than 20 non-blank lines → incomplete.
	files := fstest.MapFS{
		".github/copilot-instructions.md": {Data: []byte("# Copilot\n\nshort.\n")},
	}
	got := runSignal(t, CopilotInstructions{}, files)
	if got.FixHint == "" {
		t.Errorf("partial copilot-instructions has empty FixHint")
	}
	if len(got.Notes) == 0 {
		t.Errorf("partial copilot-instructions has empty Notes")
	}
}

func TestPRTemplate_MissingPopulatesFix(t *testing.T) {
	got := runSignal(t, PRTemplate{}, fstest.MapFS{})
	if got.FixHint == "" {
		t.Errorf("missing PR template has empty FixHint; user has nothing to act on")
	}
}

func TestCommitRules_MissingPopulatesFix(t *testing.T) {
	got := runSignal(t, CommitRules{}, fstest.MapFS{})
	if got.FixHint == "" {
		t.Errorf("missing commit-rules has empty FixHint")
	}
}

func TestContributorGuide_MissingPopulatesFix(t *testing.T) {
	got := runSignal(t, ContributorGuide{}, fstest.MapFS{})
	if got.FixHint == "" {
		t.Errorf("missing contributor-guide has empty FixHint")
	}
}
