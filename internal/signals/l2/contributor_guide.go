package l2

import (
	"context"
	"errors"
	"io/fs"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// contributorGuidePaths are the locations checked for a contributor
// guide. Listed in priority order; the first one found wins.
var contributorGuidePaths = []string{
	"CONTRIBUTING.md",
	".github/CONTRIBUTING.md",
	".github/CARD_DEVELOPMENT_GUIDE.md",
	"docs/CONTRIBUTING.md",
}

const contributorGuideLineThreshold = 20

// ContributorGuide detects whether the repo has a contributor / card
// development guide encoding common conventions and rejection reasons.
type ContributorGuide struct{}

func (ContributorGuide) ID() string        { return "l2.contributor-guide" }
func (ContributorGuide) Level() acmm.Level { return acmm.LevelInstructed }
func (ContributorGuide) Family() string    { return "instructions" }
func (ContributorGuide) Title() string {
	return "Contributor / development guide present and substantive"
}

func (s ContributorGuide) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	for _, p := range contributorGuidePaths {
		data, err := idx.Read(p)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			// Treat unexpected errors like missing for verdict math.
			continue
		}
		hasBody := countNonBlankLines(data) >= contributorGuideLineThreshold
		hasHeading := containsHeading(data)

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
				Path:    p,
				Excerpt: excerpt(data, 160),
			}},
		}
	}

	return acmm.Result{
		Status:     acmm.StatusMissing,
		Score:      acmm.ScoreMissing,
		Confidence: acmm.ConfidenceHigh,
		Method:     acmm.MethodFilenameMatch,
	}
}

func init() {
	signals.Default.Register(ContributorGuide{})
}
