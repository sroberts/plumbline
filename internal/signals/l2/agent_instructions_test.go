package l2

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sroberts/plumbline/pkg/acmm"
)

func TestAgentInstructions_Identity(t *testing.T) {
	s := AgentInstructions{}
	if s.ID() != "l2.agent-instructions" {
		t.Errorf("ID = %q, want l2.agent-instructions", s.ID())
	}
	if s.Level() != acmm.LevelInstructed {
		t.Errorf("Level = %v, want LevelInstructed", s.Level())
	}
	if s.Family() != "instructions" {
		t.Errorf("Family = %q, want instructions", s.Family())
	}
}

func TestAgentInstructions_FindsAnySupportedFile(t *testing.T) {
	body := "# Agent\n\n" + strings.Repeat("guideline.\n", 25)

	cases := []struct {
		name string
		path string
	}{
		{"CLAUDE.md", "CLAUDE.md"},
		{"AGENTS.md", "AGENTS.md"},
		{"copilot-instructions.md", ".github/copilot-instructions.md"},
		{".cursorrules", ".cursorrules"},
		{".windsurfrules", ".windsurfrules"},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			files := fstest.MapFS{c.path: {Data: []byte(body)}}
			got := runSignal(t, AgentInstructions{}, files)
			if got.Score != acmm.ScoreFound {
				t.Errorf("with %s present, score = %v, want %v",
					c.path, got.Score, acmm.ScoreFound)
			}
			if len(got.Evidence) == 0 || got.Evidence[0].Path != c.path {
				t.Errorf("Evidence should cite %s; got %+v", c.path, got.Evidence)
			}
		})
	}
}

func TestAgentInstructions_OneFileIsEnough(t *testing.T) {
	// User has CLAUDE.md but no copilot-instructions.md, no AGENTS.md.
	// Previously this would have failed l2.copilot-instructions; now
	// it should be Found because *some* agent directive is present.
	files := fstest.MapFS{
		"CLAUDE.md": {Data: []byte("# CLAUDE\n\n" + strings.Repeat("rule.\n", 25))},
	}
	got := runSignal(t, AgentInstructions{}, files)
	if got.Score != acmm.ScoreFound {
		t.Errorf("CLAUDE.md alone should be enough; score = %v", got.Score)
	}
}

func TestAgentInstructions_MissingWhenNoAgentFile(t *testing.T) {
	files := fstest.MapFS{"README.md": {Data: []byte("# r")}}
	got := runSignal(t, AgentInstructions{}, files)
	if got.Score != acmm.ScoreMissing {
		t.Errorf("no agent file present, score = %v, want missing", got.Score)
	}
	if got.FixHint == "" {
		t.Error("missing case must carry a FixHint with concrete guidance")
	}
}

func TestAgentInstructions_PartialWhenStubbed(t *testing.T) {
	// Heading present but content too thin → incomplete.
	files := fstest.MapFS{
		"CLAUDE.md": {Data: []byte("# CLAUDE\n\nshort.\n")},
	}
	got := runSignal(t, AgentInstructions{}, files)
	if got.Score != acmm.ScoreIncomplete {
		t.Errorf("heading + thin content, score = %v, want %v",
			got.Score, acmm.ScoreIncomplete)
	}
	if got.FixHint == "" {
		t.Error("partial case should carry FixHint")
	}
}

func TestAgentInstructions_PrefersFirstFound(t *testing.T) {
	// If multiple agent files exist, evidence cites one of them
	// deterministically (the first in the canonical priority list).
	body := "# x\n\n" + strings.Repeat("rule.\n", 25)
	files := fstest.MapFS{
		"AGENTS.md":                       {Data: []byte(body)},
		"CLAUDE.md":                       {Data: []byte(body)},
		".github/copilot-instructions.md": {Data: []byte(body)},
	}
	got := runSignal(t, AgentInstructions{}, files)
	if got.Score != acmm.ScoreFound {
		t.Fatalf("score = %v", got.Score)
	}
	if len(got.Evidence) == 0 {
		t.Fatal("no evidence")
	}
	// CLAUDE.md is highest priority in the canonical list.
	if got.Evidence[0].Path != "CLAUDE.md" {
		t.Errorf("Evidence[0].Path = %q, want CLAUDE.md (priority order)", got.Evidence[0].Path)
	}
}
