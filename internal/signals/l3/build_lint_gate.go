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
	// Go's own toolchain linters (go vet, staticcheck, gofmt, revive) are
	// listed alongside the third-party aggregators. A repo that lints with
	// `go vet ./...` and staticcheck — the stdlib-idiomatic pair — is doing
	// exactly what this signal asks for, and used to score as if it had no
	// linter at all because only golangci-lint was in the vocabulary.
	lintRunRE = regexp.MustCompile(`(?m)\b(lint|golangci-lint|revive|staticcheck|go vet|gofmt|goimports|eslint|prettier|stylelint|black|ruff|flake8|mypy|rubocop|clippy|biome|shellcheck)\b`)

	buildRunRE = regexp.MustCompile(`(?m)\b(go build|cargo build|go install|npm run build|yarn build|pnpm build|tsc|make build|cmake)\b`)

	lintActionRE = regexp.MustCompile(`(?i)(golangci-lint|golangci/|staticcheck-action|dominikh/staticcheck|eslint|stylelint|biomejs/biome|reviewdog)`)

	// Fallback only. SPEC.md §6 has always described this signal as
	// matching "a `lint`/`build` job name", but the AST scan read `run:`
	// bodies exclusively — a job named "lint" that shelled out to a tool
	// we did not enumerate was invisible. A name is an intent declaration
	// rather than proof, so a match here lands at low confidence.
	lintNameRE = regexp.MustCompile(`(?i)\b(lint|vet|static[- ]?analysis|fmt|format(ting)?)\b`)
)

type BuildLintGate struct{}

func (BuildLintGate) ID() string        { return "l3.build-lint-gate" }
func (BuildLintGate) Level() acmm.Level { return acmm.LevelMeasured }
func (BuildLintGate) Family() string    { return "ci-gate" }
func (BuildLintGate) Title() string     { return "CI workflow runs build and lint on push or PR" }

func (s BuildLintGate) Detect(ctx context.Context, idx *scanner.RepoIndex) acmm.Result {
	d := acmm.NewDiagnostics(ctx)
	bestScore := acmm.ScoreMissing
	var bestPath, missingHalf string

	for _, w := range idx.Workflows {
		if !d.Probe(w.Path, "trigger `push` or `pull_request`",
			w.HasPushTrigger() || w.HasPullRequestTrigger()) {
			continue
		}
		hasLint := d.Probe(w.Path, "linter command or lint action",
			w.AnyRunMatches(lintRunRE) || workflowUsesLintAction(w))
		byNameOnly := false
		if !hasLint && d.Probe(w.Path, "job/step named lint|vet|fmt (fallback)",
			w.AnyStepNameMatches(lintNameRE)) {
			hasLint, byNameOnly = true, true
		}
		hasBuild := d.Probe(w.Path, "build command", w.AnyRunMatches(buildRunRE))
		switch {
		case hasLint && hasBuild:
			res := acmm.Result{
				Status:     acmm.StatusFound,
				Score:      acmm.ScoreFound,
				Confidence: acmm.ConfidenceMedium,
				Method:     acmm.MethodAST,
				Evidence:   []acmm.Evidence{{Path: w.Path}},
			}
			if byNameOnly {
				res.Confidence = acmm.ConfidenceLow
				res.Notes = append(res.Notes,
					"lint step identified by job/step name only — no recognized linter command")
			}
			return d.Attach(res)
		case hasLint || hasBuild:
			if acmm.ScoreIncomplete > bestScore {
				bestScore = acmm.ScoreIncomplete
				bestPath = w.Path
				missingHalf = "lint"
				if hasLint {
					missingHalf = "build"
				}
			}
		}
	}

	if bestScore == acmm.ScoreMissing {
		return d.Attach(acmm.Result{
			Status:     acmm.StatusMissing,
			Score:      acmm.ScoreMissing,
			Confidence: acmm.ConfidenceMedium,
			Method:     acmm.MethodAST,
			Notes:      []string{"no push/PR-triggered workflow runs both build and lint"},
			FixHint: "Add a CI workflow on push/pull_request that builds the " +
				"project AND runs a linter (golangci-lint, eslint, etc.). " +
				"Both steps gate every PR — it's the L3 baseline.",
		})
	}
	return d.Attach(acmm.Result{
		Status:     acmm.StatusFromScore(bestScore),
		Score:      bestScore,
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodAST,
		Evidence:   []acmm.Evidence{{Path: bestPath}},
		Notes:      []string{"workflow has a " + otherHalf(missingHalf) + " step but no detected " + missingHalf + " step"},
		FixHint: "Add a " + missingHalf + " step to " + bestPath + " so every push/PR is gated on both. " +
			"If one is already there, the command isn't one this signal recognizes — " +
			"file an issue with the step, rather than reshaping CI to match the matcher.",
	})
}

func otherHalf(half string) string {
	if half == "lint" {
		return "build"
	}
	return "lint"
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
