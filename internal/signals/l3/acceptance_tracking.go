package l3

import (
	"context"
	"regexp"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

var acceptanceFileRE = regexp.MustCompile(`(?i)(auto-qa-tuning|acceptance[-_]rates?|acceptance[-_]rate)\.(json|yaml|yml)$`)
var metricsDirRE = regexp.MustCompile(`(?i)^metrics/.*\.(json|yaml|yml)$`)

type AcceptanceTracking struct{}

func (AcceptanceTracking) ID() string        { return "l3.acceptance-tracking" }
func (AcceptanceTracking) Level() acmm.Level { return acmm.LevelMeasured }
func (AcceptanceTracking) Family() string    { return "monitoring" }
func (AcceptanceTracking) Title() string {
	return "Tracked metrics file (acceptance rates / auto-qa-tuning)"
}

func (s AcceptanceTracking) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	if path, ok := anyByNameMatches(idx, acceptanceFileRE); ok {
		return acmm.Result{
			Status:     acmm.StatusFound,
			Score:      acmm.ScoreFound,
			Confidence: acmm.ConfidenceHigh,
			Method:     acmm.MethodFilenameMatch,
			Evidence:   []acmm.Evidence{{Path: path}},
		}
	}
	if path, ok := anyPathMatches(idx, metricsDirRE); ok {
		return acmm.Result{
			Status:     acmm.StatusPartial,
			Score:      acmm.ScoreIncomplete,
			Confidence: acmm.ConfidenceLow,
			Method:     acmm.MethodFilenameMatch,
			Evidence:   []acmm.Evidence{{Path: path}},
			Notes:      []string{"metrics/ file present but not the canonical name"},
		}
	}
	return acmm.Result{
		Status:     acmm.StatusMissing,
		Score:      acmm.ScoreMissing,
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodFilenameMatch,
		Notes:      []string{"no auto-qa-tuning.json / acceptance-rates.* or metrics/* file found"},
		FixHint: "Track AI agent acceptance rates: commit " +
			"auto-qa-tuning.json (or acceptance-rates.json) and update it " +
			"from a scheduled job that classifies merged-vs-rejected PRs by " +
			"category. This is what L4 self-tuning consumes.",
	}
}

func init() {
	signals.Default.Register(AcceptanceTracking{})
}
