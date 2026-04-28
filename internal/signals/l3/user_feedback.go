package l3

import (
	"context"
	"regexp"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

var (
	npsFileRE     = regexp.MustCompile(`(?i)(nps|csat|survey|feedback).*\.(ts|tsx|js|jsx|vue|py|go)$`)
	feedbackTplRE = regexp.MustCompile(`(?i)\.github/ISSUE_TEMPLATE/(feedback|nps|survey)`)
)

type UserFeedback struct{}

func (UserFeedback) ID() string        { return "l3.user-feedback" }
func (UserFeedback) Level() acmm.Level { return acmm.LevelMeasured }
func (UserFeedback) Family() string    { return "feedback" }
func (UserFeedback) Title() string     { return "User-feedback / NPS / survey channel wired up" }

func (s UserFeedback) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	if path, ok := anyPathMatches(idx, feedbackTplRE); ok {
		return acmm.Result{
			Status:     acmm.StatusFound,
			Score:      acmm.ScoreFound,
			Confidence: acmm.ConfidenceMedium,
			Method:     acmm.MethodFilenameMatch,
			Evidence:   []acmm.Evidence{{Path: path}},
		}
	}
	if path, ok := anyByNameMatches(idx, npsFileRE); ok {
		return acmm.Result{
			Status:     acmm.StatusFound,
			Score:      acmm.ScoreFound,
			Confidence: acmm.ConfidenceMedium,
			Method:     acmm.MethodFilenameMatch,
			Evidence:   []acmm.Evidence{{Path: path}},
		}
	}
	return acmm.Result{
		Status:     acmm.StatusMissing,
		Score:      acmm.ScoreMissing,
		Confidence: acmm.ConfidenceLow,
		Method:     acmm.MethodFilenameMatch,
		Notes:      []string{"low confidence — name match only"},
	}
}

func init() {
	signals.Default.Register(UserFeedback{})
}
