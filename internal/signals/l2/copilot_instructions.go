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

const (
	copilotPath          = ".github/copilot-instructions.md"
	copilotLineThreshold = 20
)

type CopilotInstructions struct{}

func (CopilotInstructions) ID() string        { return "l2.copilot-instructions" }
func (CopilotInstructions) Level() acmm.Level { return acmm.LevelInstructed }
func (CopilotInstructions) Family() string    { return "instructions" }
func (CopilotInstructions) Title() string {
	return "GitHub Copilot instructions present and substantive"
}

func (s CopilotInstructions) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	data, err := idx.Read(copilotPath)
	if err != nil {
		notes := []string{}
		if errors.Is(err, fs.ErrNotExist) {
			notes = append(notes, "no .github/copilot-instructions.md found")
		} else {
			notes = append(notes, "could not read "+copilotPath+": "+err.Error())
		}
		return acmm.Result{
			Status:     acmm.StatusMissing,
			Score:      acmm.ScoreMissing,
			Confidence: acmm.ConfidenceHigh,
			Method:     acmm.MethodContentRegex,
			Notes:      notes,
			FixHint: "Create .github/copilot-instructions.md describing your " +
				"team's conventions: tech stack, patterns to follow, anti-" +
				"patterns to avoid. Aim for ≥20 substantive lines under at " +
				"least one heading.",
		}
	}

	hasHeading := containsHeading(data)
	nonBlank := countNonBlankLines(data)
	hasBody := nonBlank >= copilotLineThreshold

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
			Path:    copilotPath,
			Excerpt: excerpt(data, 160),
		}},
	}

	if score == acmm.ScoreFound {
		res.Notes = []string{fmt.Sprintf("heading present and %d non-blank lines (≥%d)", nonBlank, copilotLineThreshold)}
		return res
	}

	if !hasHeading {
		res.Notes = append(res.Notes, "no markdown heading detected")
	}
	if !hasBody {
		res.Notes = append(res.Notes, fmt.Sprintf("only %d non-blank lines (need ≥%d for Found)", nonBlank, copilotLineThreshold))
	}
	switch {
	case !hasHeading && !hasBody:
		res.FixHint = "Add a heading and expand the file with concrete conventions; the current content is too short to function as guidance."
	case !hasHeading:
		res.FixHint = "Add a top-level markdown heading at the start of the file."
	case !hasBody:
		res.FixHint = fmt.Sprintf("Expand the file from %d to ≥%d non-blank lines covering your team's conventions and gotchas.", nonBlank, copilotLineThreshold)
	}
	return res
}

func init() {
	signals.Default.Register(CopilotInstructions{})
}
