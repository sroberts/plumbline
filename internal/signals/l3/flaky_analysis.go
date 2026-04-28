package l3

import (
	"context"
	"regexp"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

var flakyFileRE = regexp.MustCompile(`(?i)flaky[-_]?(tests?|analysis)\.(json|yaml|yml|md)$`)
var flakyWorkflowRE = regexp.MustCompile(`(?i)flaky`)

type FlakyAnalysis struct{}

func (FlakyAnalysis) ID() string        { return "l3.flaky-analysis" }
func (FlakyAnalysis) Level() acmm.Level { return acmm.LevelMeasured }
func (FlakyAnalysis) Family() string    { return "compliance" }
func (FlakyAnalysis) Title() string     { return "Flaky-test tracking workflow or report file" }

func (s FlakyAnalysis) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	// Tracked file like flaky-tests.json.
	if path, ok := anyByNameMatches(idx, flakyFileRE); ok {
		return acmm.Result{
			Status:     acmm.StatusFound,
			Score:      acmm.ScoreFound,
			Confidence: acmm.ConfidenceMedium,
			Method:     acmm.MethodFilenameMatch,
			Evidence:   []acmm.Evidence{{Path: path}},
		}
	}
	// Scheduled workflow with "flaky" in path or name.
	for _, w := range idx.Workflows {
		if !w.HasScheduledTrigger() {
			continue
		}
		if flakyWorkflowRE.MatchString(w.Path) || flakyWorkflowRE.MatchString(w.Name) {
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
		Method:     acmm.MethodFilenameMatch,
	}
}

func init() {
	signals.Default.Register(FlakyAnalysis{})
}
