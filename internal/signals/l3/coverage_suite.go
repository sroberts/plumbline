package l3

import (
	"context"
	"regexp"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

var coverageRunRE = regexp.MustCompile(`(?i)(coverage|cover|cov-)`)

type CoverageSuite struct{}

func (CoverageSuite) ID() string        { return "l3.coverage-suite" }
func (CoverageSuite) Level() acmm.Level { return acmm.LevelMeasured }
func (CoverageSuite) Family() string    { return "coverage" }
func (CoverageSuite) Title() string     { return "Scheduled (cron) coverage suite" }

func (s CoverageSuite) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	for _, w := range idx.Workflows {
		if !w.HasScheduledTrigger() {
			continue
		}
		if w.AnyRunMatches(coverageRunRE) || w.RawContains("coverage") {
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
		Notes:      []string{"no scheduled (cron) workflow runs the coverage suite"},
		FixHint: "Add a scheduled workflow (e.g. nightly cron) that runs your " +
			"full coverage suite. The PR-time gate catches regressions; the " +
			"scheduled run catches drift on slow-changing code paths.",
	}
}

func init() {
	signals.Default.Register(CoverageSuite{})
}
