package l2

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

const claudeMDPath = "CLAUDE.md"
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
		notes := []string{}
		if !errors.Is(err, fs.ErrNotExist) {
			notes = append(notes, "could not read CLAUDE.md: "+err.Error())
		} else {
			notes = append(notes, "no CLAUDE.md at repo root")
		}
		return acmm.Result{
			Status:     acmm.StatusMissing,
			Score:      acmm.ScoreMissing,
			Confidence: acmm.ConfidenceHigh,
			Method:     acmm.MethodContentRegex,
			Notes:      notes,
			FixHint: "Create a CLAUDE.md at the repo root with at least one " +
				"heading and ~30 lines of guidance covering the project's " +
				"conventions, architecture, and the kinds of changes you " +
				"want AI agents to avoid.",
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

	res := acmm.Result{
		Status:     acmm.StatusFromScore(score),
		Score:      score,
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodContentRegex,
		Evidence: []acmm.Evidence{{
			Path:    claudeMDPath,
			Excerpt: excerpt(data, 160),
		}},
	}

	if score == acmm.ScoreFound {
		res.Notes = []string{fmt.Sprintf("heading present and %d non-blank lines (≥%d)", nonBlank, claudeMDLineThreshold)}
		return res
	}

	// Partial — explain why and offer a fix.
	if !hasHeading {
		res.Notes = append(res.Notes, "no markdown heading (a line starting with '#') detected")
	}
	if !hasBody {
		res.Notes = append(res.Notes, fmt.Sprintf("only %d non-blank lines (need ≥%d for Found)", nonBlank, claudeMDLineThreshold))
	}
	switch {
	case !hasHeading && !hasBody:
		res.FixHint = "Add a markdown heading (e.g. '# CLAUDE.md') and " +
			"flesh out the file with at least ~30 lines of project " +
			"conventions and rules."
	case !hasHeading:
		res.FixHint = "Add a top-level markdown heading (e.g. '# CLAUDE.md') so the file is parseable as a structured guide."
	case !hasBody:
		res.FixHint = fmt.Sprintf("Expand CLAUDE.md from %d to ≥%d non-blank lines. Cover conventions, architecture, common pitfalls, and what *not* to do.", nonBlank, claudeMDLineThreshold)
	}
	return res
}

func init() {
	signals.Default.Register(ClaudeMD{})
}
