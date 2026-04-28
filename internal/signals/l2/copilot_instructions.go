package l2

import (
	"context"
	"errors"
	"io/fs"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

const (
	copilotPath          = ".github/copilot-instructions.md"
	copilotLineThreshold = 20
)

// CopilotInstructions detects whether .github/copilot-instructions.md
// exists with a heading and at least 20 non-blank lines. The same
// rubric shape as ClaudeMD; just a lower line bar because Copilot
// instructions tend to be shorter.
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
			Notes:      []string{"could not read " + copilotPath + ": " + err.Error()},
		}
	}

	hasHeading := containsHeading(data)
	hasBody := countNonBlankLines(data) >= copilotLineThreshold

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
			Path:    copilotPath,
			Excerpt: excerpt(data, 160),
		}},
	}
}

func init() {
	signals.Default.Register(CopilotInstructions{})
}
