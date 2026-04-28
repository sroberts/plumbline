// Package l4 holds Level 4 (Adaptive) signals — self-modifying
// configs, automated triage loops, threshold-driven blocks. Most L4
// detection is workflow-AST analysis.
package l4

import (
	"context"
	"regexp"
	"strings"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/internal/workflows"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// ===== l4.self-modifying-config =====
//
// A workflow that writes back to the repo (creates a PR or pushes
// directly). The clearest signal is use of peter-evans/create-pull-request,
// which is by far the most common pattern. Falls back to detecting a
// step that runs `git commit` + `git push`.

var gitPushRE = regexp.MustCompile(`(?m)git\s+push`)
var gitCommitRE = regexp.MustCompile(`(?m)git\s+commit`)

type SelfModifyingConfig struct{}

func (SelfModifyingConfig) ID() string        { return "l4.self-modifying-config" }
func (SelfModifyingConfig) Level() acmm.Level { return acmm.LevelAdaptive }
func (SelfModifyingConfig) Family() string    { return "automation" }
func (SelfModifyingConfig) Title() string     { return "Workflow writes back to the repo (PR or push)" }

func (SelfModifyingConfig) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	for _, w := range idx.Workflows {
		if w.UsesAction("peter-evans/create-pull-request") ||
			w.UsesAction("stefanzweifel/git-auto-commit-action") {
			return acmm.Result{
				Status:     acmm.StatusFound,
				Score:      acmm.ScoreFound,
				Confidence: acmm.ConfidenceHigh,
				Method:     acmm.MethodAST,
				Evidence:   []acmm.Evidence{{Path: w.Path}},
			}
		}
		if w.AnyRunMatches(gitPushRE) && w.AnyRunMatches(gitCommitRE) {
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
		Notes:      []string{"no workflow uses peter-evans/create-pull-request, git-auto-commit-action, or git push+commit from a step"},
		FixHint:    "Add a workflow that, on a metric or schedule, opens a PR back to the repo (peter-evans/create-pull-request is the canonical action). This is the L4 unlock — humans go from execution to governance.",
	}
}

// ===== l4.auto-triage =====
//
// A scheduled workflow that runs more than once per day and uses the
// GitHub issues API (gh issue, actions/github-script with issues, or
// peter-evans/issue-management).

var ghIssueRE = regexp.MustCompile(`(?m)\bgh\s+issue\b`)
var issuesAPIRE = regexp.MustCompile(`(?m)/issues|issues\.create|issues\.update|issues\.list`)

type AutoTriage struct{}

func (AutoTriage) ID() string        { return "l4.auto-triage" }
func (AutoTriage) Level() acmm.Level { return acmm.LevelAdaptive }
func (AutoTriage) Family() string    { return "automation" }
func (AutoTriage) Title() string     { return "Scheduled workflow that triages issues automatically" }

func (AutoTriage) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	for _, w := range idx.Workflows {
		if !w.HasScheduledTrigger() {
			continue
		}
		if !subDailySchedule(w) {
			continue
		}
		if w.AnyRunMatches(ghIssueRE) || w.AnyRunMatches(issuesAPIRE) {
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
		Notes:      []string{"no sub-daily scheduled workflow that touches the GitHub Issues API"},
		FixHint:    "Add a workflow on a sub-daily cron (e.g. '*/15 * * * *') that runs 'gh issue list' or hits /issues via actions/github-script and triages new issues automatically.",
	}
}

// subDailySchedule reports whether any cron entry runs more than once
// per day (i.e., minute or hour field is not a fixed single value).
func subDailySchedule(w *workflows.File) bool {
	for _, cron := range w.CronEntries() {
		fields := strings.Fields(cron)
		if len(fields) != 5 {
			continue
		}
		// fields = [minute, hour, day-of-month, month, day-of-week]
		// Sub-daily means hour field is a wildcard or step expression.
		if strings.Contains(fields[1], "*") || strings.Contains(fields[1], "/") || strings.Contains(fields[1], ",") {
			return true
		}
	}
	return false
}

// ===== l4.threshold-block =====
//
// A workflow conditional that reads from a metrics file and fails based
// on a numeric threshold (`if: fromJson(...).rate < N` or similar).

var thresholdIfRE = regexp.MustCompile(`(?m)if:\s*.*(?:fromJson|fromjson|<|>|<=|>=).*\d`)

type ThresholdBlock struct{}

func (ThresholdBlock) ID() string        { return "l4.threshold-block" }
func (ThresholdBlock) Level() acmm.Level { return acmm.LevelAdaptive }
func (ThresholdBlock) Family() string    { return "automation" }
func (ThresholdBlock) Title() string {
	return "Workflow conditional reads metrics and gates on a threshold"
}

func (ThresholdBlock) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	for _, w := range idx.Workflows {
		if thresholdIfRE.Match(w.Raw) {
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
		Notes:      []string{"no workflow has an `if:` conditional reading a metric and gating on a numeric threshold"},
		FixHint:    "Add a step with `if: ${{ fromJson(steps.metrics.outputs.data).rate < 80 }}` (or similar) so the system blocks itself when a metric crosses a threshold. This is what closes the L4 loop.",
	}
}

// ===== l4.worktree-agents =====
//
// Configuration enabling concurrent AI agent runs: .devcontainer/,
// .claude/, .github/agents/, or scripts that spawn worktrees.

var worktreeMarkerPaths = []string{
	".devcontainer/devcontainer.json",
	".claude/settings.json",
	".claude/agents",
}
var worktreeRE = regexp.MustCompile(`(?i)git\s+worktree`)

type WorktreeAgents struct{}

func (WorktreeAgents) ID() string        { return "l4.worktree-agents" }
func (WorktreeAgents) Level() acmm.Level { return acmm.LevelAdaptive }
func (WorktreeAgents) Family() string    { return "automation" }
func (WorktreeAgents) Title() string {
	return "Concurrent AI agent runner / devcontainer / worktree config"
}

func (WorktreeAgents) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	for _, p := range worktreeMarkerPaths {
		if _, err := idx.Read(p); err == nil {
			return acmm.Result{
				Status:     acmm.StatusFound,
				Score:      acmm.ScoreFound,
				Confidence: acmm.ConfidenceMedium,
				Method:     acmm.MethodFilenameMatch,
				Evidence:   []acmm.Evidence{{Path: p}},
			}
		}
	}
	// Search any tracked file for `git worktree` references.
	for _, f := range idx.Files {
		if strings.HasPrefix(f.Path, ".github/") || strings.HasPrefix(f.Path, "scripts/") {
			data, err := idx.Read(f.Path)
			if err != nil {
				continue
			}
			if worktreeRE.Match(data) {
				return acmm.Result{
					Status:     acmm.StatusFound,
					Score:      acmm.ScoreFound,
					Confidence: acmm.ConfidenceLow,
					Method:     acmm.MethodContentRegex,
					Evidence:   []acmm.Evidence{{Path: f.Path}},
				}
			}
		}
	}
	return acmm.Result{
		Status:     acmm.StatusMissing,
		Score:      acmm.ScoreMissing,
		Confidence: acmm.ConfidenceLow,
		Method:     acmm.MethodFilenameMatch,
		Notes:      []string{"no .devcontainer / .claude config / git-worktree scripts found"},
		FixHint:    "Set up infrastructure for concurrent AI agent runs: a .devcontainer/ for reproducible environments, or a script under scripts/ that spawns isolated git worktrees so multiple agents can work in parallel without stepping on each other.",
	}
}

// ===== l4.error-recovery =====
//
// Workflows that retry on failure (continue-on-error + retry, or
// nick-fields/retry action).

var continueOnErrorRE = regexp.MustCompile(`(?m)continue-on-error:\s*true`)

type ErrorRecovery struct{}

func (ErrorRecovery) ID() string        { return "l4.error-recovery" }
func (ErrorRecovery) Level() acmm.Level { return acmm.LevelAdaptive }
func (ErrorRecovery) Family() string    { return "automation" }
func (ErrorRecovery) Title() string {
	return "Workflow retries failures with backoff or continue-on-error"
}

func (ErrorRecovery) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	for _, w := range idx.Workflows {
		if w.UsesAction("nick-fields/retry") {
			return acmm.Result{
				Status:     acmm.StatusFound,
				Score:      acmm.ScoreFound,
				Confidence: acmm.ConfidenceHigh,
				Method:     acmm.MethodAST,
				Evidence:   []acmm.Evidence{{Path: w.Path}},
			}
		}
		if continueOnErrorRE.Match(w.Raw) {
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
		Method:     acmm.MethodAST,
		Notes:      []string{"no workflow uses nick-fields/retry or has continue-on-error: true"},
		FixHint:    "Wrap flake-prone steps with nick-fields/retry@v3 (max_attempts: 3) so the autonomous loop doesn't fail on a transient hiccup. Use `continue-on-error: true` for non-blocking diagnostics.",
	}
}

func init() {
	signals.Default.Register(SelfModifyingConfig{})
	signals.Default.Register(AutoTriage{})
	signals.Default.Register(ThresholdBlock{})
	signals.Default.Register(WorktreeAgents{})
	signals.Default.Register(ErrorRecovery{})
}
