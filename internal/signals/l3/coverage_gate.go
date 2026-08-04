package l3

import (
	"bytes"
	"context"
	"regexp"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/internal/workflows"
	"github.com/sroberts/plumbline/pkg/acmm"
)

var (
	coverageThresholdRE = regexp.MustCompile(`(?i)(--cov-fail-under|--cov-min|--minimum-coverage|--fail-under|coverage[\s_-]*threshold|fail_below|fail-below)`)
	codecovTargetRE     = regexp.MustCompile(`(?m)^\s*target:\s*\S+`)

	// The Go arm. Go has no built-in coverage-threshold flag, so every Go
	// coverage gate is hand-rolled: measure with `go tool cover -func`,
	// compare the total against a floor, exit non-zero. None of the flags
	// above can ever appear in that shape, which meant a real gate scored
	// the same as no gate at all.
	goCoverFuncRE = regexp.MustCompile(`go tool cover\s+-func`)

	// Where the floor is written down: an env var, a committed floor file,
	// or a bare literal next to a comparison.
	coverageFloorRE = regexp.MustCompile(`(?i)(coverage[\s_-]*(threshold|floor|min(imum)?)|(min|floor|threshold)[\s_-]*coverage)`)

	// The comparison itself, in the three forms CI actually uses: a shell
	// numeric test, awk/bc arithmetic, or a GitHub-native `if:` expression.
	numericCompareRE = regexp.MustCompile(`(-lt|-le|-gt|-ge|<=?|>=?)\s*\$?[({"']?\s*[0-9]`)

	// Proof the comparison gates rather than merely reports.
	failsBuildRE = regexp.MustCompile(`(?m)(exit\s+1|::error::|\bfalse\s*$)`)

	// Guards the generic threshold flags against matching a threshold that
	// has nothing to do with coverage.
	coverageContextRE = regexp.MustCompile(`(?i)(coverage|--cov|\bcover\b|lcov|\.lcov|cobertura)`)
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

	// 2. PR-triggered workflow with a coverage threshold flag in a run step
	// that is also about coverage. The context check matters: `--fail-below`
	// and friends are generic gate flags that other tools use for unrelated
	// thresholds — plumbline's own `--fail-below 3` maturity gate matched
	// here and scored the repo a coverage gate it did not have.
	for _, w := range idx.Workflows {
		if !w.HasPullRequestTrigger() {
			continue
		}
		for _, s := range w.AllSteps() {
			if s.Run == "" || !coverageThresholdRE.MatchString(s.Run) {
				continue
			}
			if !coverageContextRE.MatchString(s.Run) && !envMatchesRE(s.Env, coverageContextRE) {
				continue
			}
			return acmm.Result{
				Status:     acmm.StatusFound,
				Score:      acmm.ScoreFound,
				Confidence: acmm.ConfidenceMedium,
				Method:     acmm.MethodAST,
				Evidence:   []acmm.Evidence{{Path: w.Path}},
			}
		}
	}

	// 3. Hand-rolled Go coverage gate in a PR-triggered workflow.
	for _, w := range idx.Workflows {
		if !w.HasPullRequestTrigger() {
			continue
		}
		if note, ok := goCoverageGate(w); ok {
			return acmm.Result{
				Status:     acmm.StatusFound,
				Score:      acmm.ScoreFound,
				Confidence: acmm.ConfidenceMedium,
				Method:     acmm.MethodAST,
				Evidence:   []acmm.Evidence{{Path: w.Path}},
				Notes:      []string{note},
			}
		}
	}

	// 4. Any workflow that mentions a coverage tool but no threshold —
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

// goCoverageGate recognizes the two shapes a hand-rolled Go coverage gate
// actually takes, and returns the note to attach as evidence of which.
//
// Both require three things — the coverage is measured, it is compared
// against a floor, and losing the comparison fails the job. A workflow
// that measures coverage and merely prints it is the dashboard-graveyard
// anti-pattern, and still scores partial below.
func goCoverageGate(w *workflows.File) (string, bool) {
	// Shape A: one step does all three. The common case, because
	// `go tool cover -func` output is easiest to handle where it is
	// produced — pipe to awk, compare, exit 1.
	for _, s := range w.AllSteps() {
		if s.Run == "" || !goCoverFuncRE.MatchString(s.Run) {
			continue
		}
		hasFloor := coverageFloorRE.MatchString(s.Run) ||
			numericCompareRE.MatchString(s.Run) ||
			envMatchesRE(s.Env, coverageFloorRE)
		if hasFloor && failsBuildRE.MatchString(s.Run) {
			return "hand-rolled Go coverage gate: go tool cover -func compared against a floor, exits non-zero below it", true
		}
	}

	// Shape B: measurement, comparison, and failure are split across
	// steps, with the comparison in a GitHub-native `if:` expression and
	// the run body reduced to the error message.
	if !w.AnyRunMatches(goCoverFuncRE) {
		return "", false
	}
	if !(w.AnyRunMatches(coverageFloorRE) || w.AnyEnvMatches(coverageFloorRE)) {
		return "", false
	}
	for _, s := range w.AllSteps() {
		if s.If != "" && numericCompareRE.MatchString(s.If) && failsBuildRE.MatchString(s.Run) {
			return "hand-rolled Go coverage gate: threshold comparison in an if: conditional that fails the job", true
		}
	}
	return "", false
}

func envMatchesRE(env map[string]string, re *regexp.Regexp) bool {
	for k, v := range env {
		if re.MatchString(k) || re.MatchString(v) {
			return true
		}
	}
	return false
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
