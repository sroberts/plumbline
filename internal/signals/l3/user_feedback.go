package l3

import (
	"context"
	"regexp"
	"strings"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

var (
	// npsFileRE matches files whose basename contains a feedback-channel
	// keyword in a frontend-language extension. The path constraint
	// (userFeedbackPathRE) is what actually rules out false positives;
	// this regex stays loose so legitimate names like `useNPSSurvey.ts`
	// or `userFeedbackForm.tsx` still match.
	npsFileRE = regexp.MustCompile(`(?i)(nps|csat|survey|feedback).*\.(ts|tsx|js|jsx|vue|py|go|rb|kt|swift)$`)

	// feedbackTplRE matches GitHub issue templates dedicated to feedback.
	feedbackTplRE = regexp.MustCompile(`(?i)\.github/ISSUE_TEMPLATE/(feedback|nps|survey)`)

	// userFeedbackPathRE constrains npsFileRE matches to common app paths
	// (web/src, src, app, frontend, ui, components). Plumbline's own source
	// at internal/signals/l3/user_feedback.go was matching the legacy regex
	// — this ensures the signal only fires for genuine user-facing
	// feedback components, not for files that happen to have "feedback"
	// in the name in any directory.
	userFeedbackPathRE = regexp.MustCompile(`(?i)^(web|src|app|frontend|ui|components|pages|hooks|routes|lib)/`)
)

type UserFeedback struct{}

func (UserFeedback) ID() string        { return "l3.user-feedback" }
func (UserFeedback) Level() acmm.Level { return acmm.LevelMeasured }
func (UserFeedback) Family() string    { return "feedback" }
func (UserFeedback) Title() string     { return "User-feedback / NPS / survey channel wired up" }

func (s UserFeedback) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	// 1. A dedicated GitHub issue template for feedback / NPS / survey.
	//    High-confidence: directory + filename together imply intent.
	if path, ok := anyPathMatches(idx, feedbackTplRE); ok {
		return acmm.Result{
			Status:     acmm.StatusFound,
			Score:      acmm.ScoreFound,
			Confidence: acmm.ConfidenceMedium,
			Method:     acmm.MethodFilenameMatch,
			Evidence:   []acmm.Evidence{{Path: path}},
		}
	}

	// 2. An NPS/survey component in a *frontend-shaped* path. Both
	//    constraints together (basename + parent dir) make false positives
	//    much rarer.
	for _, f := range idx.Files {
		base := basename(f.Path)
		if !npsFileRE.MatchString(base) {
			continue
		}
		if !userFeedbackPathRE.MatchString(f.Path) {
			continue
		}
		return acmm.Result{
			Status:     acmm.StatusFound,
			Score:      acmm.ScoreFound,
			Confidence: acmm.ConfidenceMedium,
			Method:     acmm.MethodFilenameMatch,
			Evidence:   []acmm.Evidence{{Path: f.Path}},
		}
	}

	return acmm.Result{
		Status:     acmm.StatusMissing,
		Score:      acmm.ScoreMissing,
		Confidence: acmm.ConfidenceLow,
		Method:     acmm.MethodFilenameMatch,
		Notes: []string{
			"no NPS / survey component or feedback issue template found",
			"low confidence — name match only",
		},
		FixHint: "Add a lightweight user-feedback channel: an NPS / CSAT " +
			"survey component (e.g. web/src/hooks/useNPSSurvey.ts), or a " +
			".github/ISSUE_TEMPLATE/feedback.md so users can report what " +
			"shipped wrong.",
	}
}

func basename(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

func init() {
	signals.Default.Register(UserFeedback{})
}
