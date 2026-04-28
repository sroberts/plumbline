package l3

import (
	"context"
	"regexp"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/internal/workflows"
	"github.com/sroberts/plumbline/pkg/acmm"
)

var (
	lintRunRE    = regexp.MustCompile(`(?m)\b(lint|golangci-lint|eslint|prettier|stylelint|black|ruff|flake8|rubocop|clippy|biome|shellcheck)\b`)
	buildRunRE   = regexp.MustCompile(`(?m)\b(go build|cargo build|go install|npm run build|yarn build|pnpm build|tsc|make build|cmake)\b`)
	lintActionRE = regexp.MustCompile(`(?i)(golangci-lint|eslint|stylelint|biomejs/biome|reviewdog)`)
)

type BuildLintGate struct{}

func (BuildLintGate) ID() string        { return "l3.build-lint-gate" }
func (BuildLintGate) Level() acmm.Level { return acmm.LevelMeasured }
func (BuildLintGate) Family() string    { return "ci-gate" }
func (BuildLintGate) Title() string     { return "CI workflow runs build and lint on push or PR" }

func (s BuildLintGate) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	bestScore := acmm.ScoreMissing
	var bestPath string

	for _, w := range idx.Workflows {
		if !(w.HasPushTrigger() || w.HasPullRequestTrigger()) {
			continue
		}
		hasLint := w.AnyRunMatches(lintRunRE) || workflowUsesLintAction(w)
		hasBuild := w.AnyRunMatches(buildRunRE)
		switch {
		case hasLint && hasBuild:
			return acmm.Result{
				Status:     acmm.StatusFound,
				Score:      acmm.ScoreFound,
				Confidence: acmm.ConfidenceMedium,
				Method:     acmm.MethodAST,
				Evidence:   []acmm.Evidence{{Path: w.Path}},
			}
		case hasLint || hasBuild:
			if acmm.ScoreIncomplete > bestScore {
				bestScore = acmm.ScoreIncomplete
				bestPath = w.Path
			}
		}
	}

	if bestScore == acmm.ScoreMissing {
		return acmm.Result{
			Status:     acmm.StatusMissing,
			Score:      acmm.ScoreMissing,
			Confidence: acmm.ConfidenceMedium,
			Method:     acmm.MethodAST,
			Notes:      []string{"no push/PR-triggered workflow runs both build and lint"},
			FixHint: "Add a CI workflow on push/pull_request that builds the " +
				"project AND runs a linter (golangci-lint, eslint, etc.). " +
				"Both steps gate every PR — it's the L3 baseline.",
		}
	}
	return acmm.Result{
		Status:     acmm.StatusFromScore(bestScore),
		Score:      bestScore,
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodAST,
		Evidence:   []acmm.Evidence{{Path: bestPath}},
		Notes:      []string{"workflow runs only one of build / lint"},
		FixHint:    "Extend the existing CI workflow so it runs both a build step and a lint step on every push/PR.",
	}
}

func workflowUsesLintAction(w *workflows.File) bool {
	for _, j := range w.Jobs {
		for _, st := range j.Steps {
			if lintActionRE.MatchString(st.Uses) {
				return true
			}
		}
	}
	return false
}

func init() {
	signals.Default.Register(BuildLintGate{})
}
