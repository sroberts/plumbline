// Package l5 holds Level 5 (Self-Sustaining) signals — the codebase
// is the policy. Issue-to-PR pipelines, self-improvement loops,
// docs-from-PR sync, multi-repo orchestration.
package l5

import (
	"context"
	"regexp"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/internal/workflows"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// ===== l5.issue-to-pr =====
//
// A workflow triggered by issues:[opened|labeled] that opens a PR.

type IssueToPR struct{}

func (IssueToPR) ID() string        { return "l5.issue-to-pr" }
func (IssueToPR) Level() acmm.Level { return acmm.LevelSelfSustaining }
func (IssueToPR) Family() string    { return "automation" }
func (IssueToPR) Title() string     { return "Issues open PRs automatically" }

func (IssueToPR) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	for _, w := range idx.Workflows {
		if !w.HasIssuesTrigger() {
			continue
		}
		if !(w.IssuesTriggerHasType("opened") || w.IssuesTriggerHasType("labeled")) {
			continue
		}
		if opensPR(w) {
			return acmm.Result{
				Status:     acmm.StatusFound,
				Score:      acmm.ScoreFound,
				Confidence: acmm.ConfidenceHigh,
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
	}
}

func opensPR(w *workflows.File) bool {
	if w.UsesAction("peter-evans/create-pull-request") {
		return true
	}
	createPRRE := regexp.MustCompile(`(?m)gh\s+pr\s+create`)
	return w.AnyRunMatches(createPRRE)
}

// ===== l5.self-improvement =====
//
// A workflow on PR-merge that writes back to instruction files
// (CLAUDE.md, .github/copilot-instructions.md, etc.).

var instructionFilesRE = regexp.MustCompile(`(?m)CLAUDE\.md|copilot-instructions|CARD_DEVELOPMENT_GUIDE`)

type SelfImprovement struct{}

func (SelfImprovement) ID() string        { return "l5.self-improvement" }
func (SelfImprovement) Level() acmm.Level { return acmm.LevelSelfSustaining }
func (SelfImprovement) Family() string    { return "automation" }
func (SelfImprovement) Title() string {
	return "Workflow updates instruction files based on merged PRs"
}

func (SelfImprovement) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	for _, w := range idx.Workflows {
		if !w.PullRequestClosed() {
			continue
		}
		if instructionFilesRE.Match(w.Raw) {
			return acmm.Result{
				Status:     acmm.StatusFound,
				Score:      acmm.ScoreFound,
				Confidence: acmm.ConfidenceMedium,
				Method:     acmm.MethodContentRegex,
				Evidence:   []acmm.Evidence{{Path: w.Path}},
			}
		}
	}
	return acmm.Result{
		Status:     acmm.StatusMissing,
		Score:      acmm.ScoreMissing,
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodContentRegex,
	}
}

// ===== l5.docs-from-prs =====
//
// A workflow triggered on PR events that updates documentation
// (commits to docs/, README, web/docs).

var docsPathRE = regexp.MustCompile(`(?m)(docs/|README\.md|web/docs/)`)

type DocsFromPRs struct{}

func (DocsFromPRs) ID() string        { return "l5.docs-from-prs" }
func (DocsFromPRs) Level() acmm.Level { return acmm.LevelSelfSustaining }
func (DocsFromPRs) Family() string    { return "automation" }
func (DocsFromPRs) Title() string     { return "Documentation updates triggered by PR events" }

func (DocsFromPRs) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	for _, w := range idx.Workflows {
		if !w.HasPullRequestTrigger() {
			continue
		}
		if docsPathRE.Match(w.Raw) && (w.UsesAction("peter-evans/create-pull-request") || w.AnyRunMatches(regexp.MustCompile(`(?m)git\s+commit`))) {
			return acmm.Result{
				Status:     acmm.StatusFound,
				Score:      acmm.ScoreFound,
				Confidence: acmm.ConfidenceMedium,
				Method:     acmm.MethodContentRegex,
				Evidence:   []acmm.Evidence{{Path: w.Path}},
			}
		}
	}
	return acmm.Result{
		Status:     acmm.StatusMissing,
		Score:      acmm.ScoreMissing,
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodContentRegex,
	}
}

// ===== l5.multi-repo-orchestration =====
//
// A workflow that fans out across repositories — either via `gh api
// repos/...` calls in run scripts, or a matrix that varies a "repo"
// dimension.

var multiRepoRE = regexp.MustCompile(`(?m)gh\s+api\s+repos/[^\s]+/[^\s]+`)
var matrixRepoRE = regexp.MustCompile(`(?m)matrix:[\s\S]*?repo[s]?:\s*[\[\n]`)

type MultiRepoOrchestration struct{}

func (MultiRepoOrchestration) ID() string        { return "l5.multi-repo-orchestration" }
func (MultiRepoOrchestration) Level() acmm.Level { return acmm.LevelSelfSustaining }
func (MultiRepoOrchestration) Family() string    { return "automation" }
func (MultiRepoOrchestration) Title() string     { return "Workflow fans out across multiple repositories" }

func (MultiRepoOrchestration) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	for _, w := range idx.Workflows {
		if multiRepoRE.Match(w.Raw) || matrixRepoRE.Match(w.Raw) {
			return acmm.Result{
				Status:     acmm.StatusFound,
				Score:      acmm.ScoreFound,
				Confidence: acmm.ConfidenceMedium,
				Method:     acmm.MethodContentRegex,
				Evidence:   []acmm.Evidence{{Path: w.Path}},
			}
		}
	}
	return acmm.Result{
		Status:     acmm.StatusMissing,
		Score:      acmm.ScoreMissing,
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodContentRegex,
	}
}

func init() {
	signals.Default.Register(IssueToPR{})
	signals.Default.Register(SelfImprovement{})
	signals.Default.Register(DocsFromPRs{})
	signals.Default.Register(MultiRepoOrchestration{})
}
