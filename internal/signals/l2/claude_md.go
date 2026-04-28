// Package l2 holds Level 2 (Instructed) signals — encoded preferences
// in instruction files that AI agents consume at the start of every
// session. See SPEC.md §6.
package l2

import (
	"context"
	"errors"
	"io/fs"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// claudeMDPath is the file the signal looks for at the repo root.
const claudeMDPath = "CLAUDE.md"

// claudeMDLineThreshold is the bar for "substantive" content. Below it,
// the file is treated as a stub even when it has a heading.
const claudeMDLineThreshold = 30

// ClaudeMD detects whether a substantive CLAUDE.md exists at the repo
// root. Found requires both a markdown heading and ≥ 30 non-blank lines
// of content.
type ClaudeMD struct{}

func (ClaudeMD) ID() string        { return "l2.claude-md" }
func (ClaudeMD) Level() acmm.Level { return acmm.LevelInstructed }
func (ClaudeMD) Family() string    { return "instructions" }
func (ClaudeMD) Title() string     { return "CLAUDE.md present and substantive" }

func (s ClaudeMD) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	data, err := idx.Read(claudeMDPath)
	if err != nil {
		// File-not-found is the expected Missing case; any other error
		// is also Missing for now (signals don't propagate errors —
		// the verdict caller cannot do anything with them).
		if errors.Is(err, fs.ErrNotExist) {
			return acmm.Result{
				Status:     acmm.StatusMissing,
				Score:      acmm.ScoreMissing,
				Confidence: acmm.ConfidenceHigh,
				Method:     acmm.MethodContentRegex,
			}
		}
		return acmm.Result{
			Status:     acmm.StatusMissing,
			Score:      acmm.ScoreMissing,
			Confidence: acmm.ConfidenceHigh,
			Method:     acmm.MethodContentRegex,
			Notes:      []string{"could not read CLAUDE.md: " + err.Error()},
		}
	}

	hasHeading := containsHeading(data)
	nonBlank := countNonBlankLines(data)
	hasBody := nonBlank >= claudeMDLineThreshold

	score := acmm.ScoreStubbed
	switch {
	case hasHeading && hasBody:
		score = acmm.ScoreFound
	case hasHeading || hasBody:
		score = acmm.ScoreIncomplete
	}

	return acmm.Result{
		Status:     acmm.StatusFromScore(score),
		Score:      score,
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodContentRegex,
		Evidence: []acmm.Evidence{{
			Path:    claudeMDPath,
			Excerpt: excerpt(data, 160),
		}},
	}
}

func init() {
	signals.Default.Register(ClaudeMD{})
}
