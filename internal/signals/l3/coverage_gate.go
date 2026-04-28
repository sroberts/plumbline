package l3

import (
	"bytes"
	"context"
	"regexp"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

var (
	coverageThresholdRE = regexp.MustCompile(`(?i)(--cov-fail-under|--cov-min|--minimum-coverage|coverage\s+threshold|coverageThreshold|fail_below|fail-below)`)
	codecovTargetRE     = regexp.MustCompile(`(?m)^\s*target:\s*\S+`)
)

type CoverageGate struct{}

func (CoverageGate) ID() string        { return "l3.coverage-gate" }
func (CoverageGate) Level() acmm.Level { return acmm.LevelMeasured }
func (CoverageGate) Family() string    { return "coverage" }
func (CoverageGate) Title() string     { return "Coverage gate fails CI below a threshold" }

func (s CoverageGate) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	// 1. codecov.yml / .codecov.yml / .codecov.yaml with a target.
	for _, p := range []string{"codecov.yml", ".codecov.yml", ".codecov.yaml"} {
		if data := readOrEmpty(idx, p); len(data) > 0 {
			if codecovTargetRE.Match(data) {
				return acmm.Result{
					Status:     acmm.StatusFound,
					Score:      acmm.ScoreFound,
					Confidence: acmm.ConfidenceMedium,
					Method:     acmm.MethodContentRegex,
					Evidence:   []acmm.Evidence{{Path: p, Excerpt: string(bytes.TrimSpace(data[:min(160, len(data))]))}},
				}
			}
		}
	}

	// 2. PR-triggered workflow with a coverage threshold flag in any run step.
	for _, w := range idx.Workflows {
		if !w.HasPullRequestTrigger() {
			continue
		}
		if w.AnyRunMatches(coverageThresholdRE) {
			return acmm.Result{
				Status:     acmm.StatusFound,
				Score:      acmm.ScoreFound,
				Confidence: acmm.ConfidenceMedium,
				Method:     acmm.MethodAST,
				Evidence:   []acmm.Evidence{{Path: w.Path}},
			}
		}
	}

	// 3. Any workflow that mentions a coverage tool but no threshold —
	// partial credit, since the loop is wired but the gate isn't.
	for _, w := range idx.Workflows {
		if w.RawContains("coverage") || w.RawContains("--cover") {
			return acmm.Result{
				Status:     acmm.StatusPartial,
				Score:      acmm.ScoreIncomplete,
				Confidence: acmm.ConfidenceLow,
				Method:     acmm.MethodContentRegex,
				Evidence:   []acmm.Evidence{{Path: w.Path}},
				Notes:      []string{"coverage runs but no threshold flag detected"},
			}
		}
	}

	return acmm.Result{
		Status:     acmm.StatusMissing,
		Score:      acmm.ScoreMissing,
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodContentRegex,
		Notes:      []string{"no codecov.yml target and no coverage threshold flag in any PR workflow"},
		FixHint: "Either commit a codecov.yml with target.coverage (e.g. 80%), " +
			"or add a coverage threshold flag to your test step " +
			"(--cov-fail-under=80 for pytest, -coverpkg + threshold for go, etc.) " +
			"so PRs that drop coverage actually fail.",
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	signals.Default.Register(CoverageGate{})
}
