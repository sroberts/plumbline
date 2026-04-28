package l3

import (
	"context"
	"regexp"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

var nightlyKeywordRE = regexp.MustCompile(`(?i)(nightly|compliance|a11y|accessibility|security|perf|performance|smoke|e2e)`)

type NightlyCompliance struct{}

func (NightlyCompliance) ID() string        { return "l3.nightly-compliance" }
func (NightlyCompliance) Level() acmm.Level { return acmm.LevelMeasured }
func (NightlyCompliance) Family() string    { return "compliance" }
func (NightlyCompliance) Title() string     { return "Scheduled compliance / a11y / perf / security suite" }

func (s NightlyCompliance) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	for _, w := range idx.Workflows {
		if !w.HasScheduledTrigger() {
			continue
		}
		if nightlyKeywordRE.MatchString(w.Path) || nightlyKeywordRE.MatchString(w.Name) {
			return acmm.Result{
				Status:     acmm.StatusFound,
				Score:      acmm.ScoreFound,
				Confidence: acmm.ConfidenceMedium,
				Method:     acmm.MethodAST,
				Evidence:   []acmm.Evidence{{Path: w.Path}},
			}
		}
	}
	return acmm.Result{
		Status:     acmm.StatusMissing,
		Score:      acmm.ScoreMissing,
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodAST,
		Notes:      []string{"no scheduled compliance / a11y / perf / security workflow detected"},
		FixHint: "Add a nightly cron workflow named (or with a path containing) " +
			"'nightly', 'compliance', 'a11y', 'perf', or 'security'. Use it " +
			"to run the heavy checks that don't fit the per-PR latency budget.",
	}
}

func init() {
	signals.Default.Register(NightlyCompliance{})
}
