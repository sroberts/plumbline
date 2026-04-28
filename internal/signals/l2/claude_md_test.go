package l2

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/pkg/acmm"
)

func TestClaudeMD_Identity(t *testing.T) {
	s := ClaudeMD{}
	if s.ID() != "l2.claude-md" {
		t.Errorf("ID = %q, want l2.claude-md", s.ID())
	}
	if s.Level() != acmm.LevelInstructed {
		t.Errorf("Level = %v, want LevelInstructed", s.Level())
	}
	if s.Family() != "instructions" {
		t.Errorf("Family = %q, want instructions", s.Family())
	}
	if s.Title() == "" {
		t.Error("Title is empty")
	}
}

func TestClaudeMD_Detect(t *testing.T) {
	// 35 non-blank lines of "guidance" — exceeds the 30-line bar.
	longBody := strings.Repeat("Some non-trivial line of guidance.\n", 35)

	cases := []struct {
		name           string
		files          fstest.MapFS
		wantStatus     acmm.Status
		wantScore      float64
		wantConfidence acmm.Confidence
	}{
		{
			name:           "absent",
			files:          fstest.MapFS{"README.md": {Data: []byte("# r")}},
			wantStatus:     acmm.StatusMissing,
			wantScore:      acmm.ScoreMissing,
			wantConfidence: acmm.ConfidenceHigh,
		},
		{
			name: "present in subdir but not at root",
			files: fstest.MapFS{
				"docs/CLAUDE.md": {Data: []byte("# CLAUDE\n\n" + longBody)},
			},
			wantStatus:     acmm.StatusMissing,
			wantScore:      acmm.ScoreMissing,
			wantConfidence: acmm.ConfidenceHigh,
		},
		{
			name: "stubbed: no heading and few lines",
			files: fstest.MapFS{
				"CLAUDE.md": {Data: []byte("just plain text\nno markdown\nthree lines\n")},
			},
			wantStatus:     acmm.StatusPartial,
			wantScore:      acmm.ScoreStubbed,
			wantConfidence: acmm.ConfidenceMedium,
		},
		{
			name: "incomplete: heading but few lines",
			files: fstest.MapFS{
				"CLAUDE.md": {Data: []byte("# CLAUDE.md\n\nshort guide.\n")},
			},
			wantStatus:     acmm.StatusPartial,
			wantScore:      acmm.ScoreIncomplete,
			wantConfidence: acmm.ConfidenceMedium,
		},
		{
			name: "incomplete: many lines but no heading",
			files: fstest.MapFS{
				"CLAUDE.md": {Data: []byte(longBody)},
			},
			wantStatus:     acmm.StatusPartial,
			wantScore:      acmm.ScoreIncomplete,
			wantConfidence: acmm.ConfidenceMedium,
		},
		{
			name: "found: heading and >= 30 non-blank lines",
			files: fstest.MapFS{
				"CLAUDE.md": {Data: []byte("# CLAUDE.md\n\n" + longBody)},
			},
			wantStatus:     acmm.StatusFound,
			wantScore:      acmm.ScoreFound,
			wantConfidence: acmm.ConfidenceMedium,
		},
		{
			name: "blank lines do not count toward the 30-line bar",
			files: fstest.MapFS{
				// 35 lines of just newlines + a heading + 5 content lines.
				"CLAUDE.md": {Data: []byte("# CLAUDE.md\n" + strings.Repeat("\n", 35) + "a\nb\nc\nd\ne\n")},
			},
			wantStatus:     acmm.StatusPartial,
			wantScore:      acmm.ScoreIncomplete,
			wantConfidence: acmm.ConfidenceMedium,
		},
	}

	s := ClaudeMD{}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			idx, err := scanner.ScanFS(c.files, "/repo")
			if err != nil {
				t.Fatalf("ScanFS: %v", err)
			}
			got := s.Detect(context.Background(), idx)
			if got.Status != c.wantStatus {
				t.Errorf("Status = %q, want %q", got.Status, c.wantStatus)
			}
			if got.Score != c.wantScore {
				t.Errorf("Score = %v, want %v", got.Score, c.wantScore)
			}
			if got.Confidence != c.wantConfidence {
				t.Errorf("Confidence = %q, want %q", got.Confidence, c.wantConfidence)
			}
			// Method and Evidence must be set; concrete values are
			// implementation choices not pinned by these tests.
			if got.Method == "" {
				t.Error("Method is empty")
			}
			if got.Status == acmm.StatusMissing && len(got.Evidence) > 0 {
				t.Errorf("Missing should have no evidence, got %d entries", len(got.Evidence))
			}
			if got.Status != acmm.StatusMissing && len(got.Evidence) == 0 {
				t.Error("non-Missing result should carry at least one Evidence entry")
			}
		})
	}
}
