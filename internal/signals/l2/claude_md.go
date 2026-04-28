// Package l2 holds Level 2 (Instructed) signals — encoded preferences
// in instruction files that AI agents consume at the start of every
// session. See SPEC.md §6.
package l2

import (
	"bytes"
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

// containsHeading reports whether data contains at least one ATX-style
// markdown heading (# / ## / ###...) at the start of any line.
func containsHeading(data []byte) bool {
	for _, line := range bytes.Split(data, []byte("\n")) {
		trimmed := bytes.TrimLeft(line, " \t")
		if len(trimmed) > 0 && trimmed[0] == '#' {
			return true
		}
	}
	return false
}

// countNonBlankLines returns the number of lines containing at least
// one non-whitespace character.
func countNonBlankLines(data []byte) int {
	n := 0
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			n++
		}
	}
	return n
}

// excerpt returns the first n bytes of data, with a trailing ellipsis
// if truncated. Used in Evidence so the verdict has a citation.
func excerpt(data []byte, n int) string {
	if len(data) <= n {
		return string(data)
	}
	return string(data[:n]) + "…"
}

func init() {
	signals.Default.Register(ClaudeMD{})
}
