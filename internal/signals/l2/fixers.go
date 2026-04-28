package l2

import (
	"context"
	"fmt"
	"strings"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// Each L2 detector also implements signals.Fixer. The contract: if the
// target file is missing, scaffold it; if it exists but is incomplete,
// append a TODO block prompting the user to expand it. We never
// overwrite existing prose — that would be surprising and destructive.

// ===== ClaudeMD fixer =====

func (ClaudeMD) Inputs() []acmm.FixInput {
	return []acmm.FixInput{
		{
			Key:      "project_summary",
			Label:    "One-paragraph summary of this project (audience: an AI agent that just walked in)",
			Help:     "What does this project do? Who uses it? What's the tech stack?",
			Kind:     acmm.FixInputMultiline,
			Required: false,
			Default:  "TODO: replace this with a one-paragraph project summary.",
		},
		{
			Key:      "conventions",
			Label:    "Top 3-5 conventions you want AI agents to follow",
			Help:     "e.g., 'use sql/template, never raw fmt.Sprintf for queries'; 'prefer composition over inheritance'.",
			Kind:     acmm.FixInputMultiline,
			Required: false,
			Default:  "TODO: list the conventions an AI agent should follow.",
		},
		{
			Key:      "antipatterns",
			Label:    "Top 3-5 things AI agents should NOT do",
			Help:     "Past frustrations are gold here. e.g., 'don't add new packages without checking go.sum first'.",
			Kind:     acmm.FixInputMultiline,
			Required: false,
			Default:  "TODO: list the things AI agents should never do.",
		},
	}
}

func (s ClaudeMD) Plan(_ context.Context, idx *scanner.RepoIndex, inputs map[string]string) (acmm.FixPlan, error) {
	body := buildClaudeMD(inputs)
	if _, err := idx.Read(claudeMDPath); err != nil {
		// File missing — scaffold from scratch.
		return acmm.FixPlan{
			SignalID: s.ID(),
			Summary:  fmt.Sprintf("Create %s with a structured AI guidance template", claudeMDPath),
			Ops: []acmm.FixOp{{
				Kind: acmm.FixCreateFile,
				Path: claudeMDPath,
				Body: []byte(body),
			}},
		}, nil
	}
	// File exists but is incomplete — append a TODO expansion block.
	appendBody := buildClaudeMDAppend(inputs)
	return acmm.FixPlan{
		SignalID: s.ID(),
		Summary:  fmt.Sprintf("Append a guidance expansion block to %s (existing content untouched)", claudeMDPath),
		Ops: []acmm.FixOp{{
			Kind: acmm.FixAppendFile,
			Path: claudeMDPath,
			Body: []byte(appendBody),
		}},
	}, nil
}

func buildClaudeMD(inputs map[string]string) string {
	summary := getOr(inputs, "project_summary", "TODO: replace this with a one-paragraph project summary.")
	conventions := getOr(inputs, "conventions", "TODO: list the conventions an AI agent should follow.")
	antipatterns := getOr(inputs, "antipatterns", "TODO: list the things AI agents should never do.")
	return fmt.Sprintf(`# CLAUDE.md

This file provides guidance to Claude Code and other AI coding agents
working in this repository.

## Project summary

%s

## Conventions

%s

## Anti-patterns (do NOT)

%s

## Workflow

- Run the test suite before suggesting code is ready to merge.
- Match the formatting and structure of existing code.
- When unsure, ask before introducing new dependencies.

## Where to look

- Source: ./
- Tests: alongside source files (*_test.go) or under tests/.
- Docs: README.md, docs/.
`, summary, conventions, antipatterns)
}

func buildClaudeMDAppend(inputs map[string]string) string {
	conventions := getOr(inputs, "conventions", "TODO: add the conventions an AI agent should follow.")
	antipatterns := getOr(inputs, "antipatterns", "TODO: add the things AI agents should never do.")
	return fmt.Sprintf(`<!-- plumbline: appended block; expand below to reach the L2 substantive bar -->

## Additional conventions

%s

## Additional anti-patterns (do NOT)

%s
`, conventions, antipatterns)
}

// ===== CopilotInstructions fixer =====

func (CopilotInstructions) Inputs() []acmm.FixInput {
	return []acmm.FixInput{
		{
			Key:      "tech_stack",
			Label:    "Tech stack one-liner",
			Help:     "e.g., 'Go 1.24 backend, Vite + React TS frontend, Postgres'.",
			Kind:     acmm.FixInputText,
			Required: false,
			Default:  "TODO: name the languages/frameworks used here.",
		},
		{
			Key:      "rules",
			Label:    "5–10 short rules Copilot should follow",
			Help:     "Bullet points; one rule per line.",
			Kind:     acmm.FixInputMultiline,
			Required: false,
			Default:  "- TODO: add rules.",
		},
	}
}

func (s CopilotInstructions) Plan(_ context.Context, idx *scanner.RepoIndex, inputs map[string]string) (acmm.FixPlan, error) {
	stack := getOr(inputs, "tech_stack", "TODO: name the languages/frameworks used here.")
	rules := getOr(inputs, "rules", "- TODO: add rules.")
	body := fmt.Sprintf(`# GitHub Copilot instructions

## Tech stack

%s

## Rules for code completions

%s

## Test discipline

- New code requires a test (table-driven if it's a function with branches).
- Don't add a feature flag for code that isn't behind one in production.

## Style

- Match the existing code's formatting; don't reformat unrelated lines.
- Comments only when the *why* is non-obvious.
`, stack, rules)

	if _, err := idx.Read(copilotPath); err == nil {
		return acmm.FixPlan{
			SignalID: s.ID(),
			Summary:  fmt.Sprintf("Append additional rules to %s", copilotPath),
			Ops: []acmm.FixOp{{
				Kind: acmm.FixAppendFile,
				Path: copilotPath,
				Body: []byte("\n<!-- plumbline: appended block -->\n\n## Additional rules\n\n" + rules + "\n"),
			}},
		}, nil
	}
	return acmm.FixPlan{
		SignalID: s.ID(),
		Summary:  fmt.Sprintf("Create %s", copilotPath),
		Ops: []acmm.FixOp{{
			Kind: acmm.FixCreateFile,
			Path: copilotPath,
			Body: []byte(body),
		}},
	}, nil
}

// ===== ContributorGuide fixer =====

func (ContributorGuide) Inputs() []acmm.FixInput {
	return []acmm.FixInput{
		{
			Key:      "review_criteria",
			Label:    "What gets a PR rejected?",
			Help:     "The most common reasons a PR doesn't merge. AI agents will read this and avoid those mistakes.",
			Kind:     acmm.FixInputMultiline,
			Required: false,
			Default:  "TODO: list common rejection reasons.",
		},
	}
}

func (s ContributorGuide) Plan(_ context.Context, idx *scanner.RepoIndex, inputs map[string]string) (acmm.FixPlan, error) {
	// Default location: CONTRIBUTING.md at repo root.
	target := "CONTRIBUTING.md"
	for _, p := range contributorGuidePaths {
		if _, err := idx.Read(p); err == nil {
			target = p
			break
		}
	}
	criteria := getOr(inputs, "review_criteria", "TODO: list common rejection reasons.")
	body := fmt.Sprintf(`# Contributing

Welcome. This guide is the canonical source for how PRs get reviewed
and merged in this repository — both for human contributors and for
AI coding agents.

## Workflow

1. Branch from main: `+"`"+`git checkout -b <type>/<short-name>`+"`"+`.
2. Make focused commits — one logical change per PR.
3. Run the full test suite locally before opening the PR.
4. Open a PR; the template lists the per-merge checks.

## What gets a PR rejected

%s

## Style

- Format with the language's standard tool (gofmt, prettier, etc.).
- Don't reformat unrelated lines; keep diffs minimal.
- Comments explain WHY, not WHAT.

## Asking for help

Open a draft PR or an issue. Don't sit on a stuck branch.
`, criteria)

	if _, err := idx.Read(target); err == nil {
		// File exists; append.
		return acmm.FixPlan{
			SignalID: s.ID(),
			Summary:  fmt.Sprintf("Append additional criteria to %s", target),
			Ops: []acmm.FixOp{{
				Kind: acmm.FixAppendFile,
				Path: target,
				Body: []byte("\n<!-- plumbline: appended block -->\n\n## Additional review criteria\n\n" + criteria + "\n"),
			}},
		}, nil
	}
	return acmm.FixPlan{
		SignalID: s.ID(),
		Summary:  fmt.Sprintf("Create %s", target),
		Ops: []acmm.FixOp{{
			Kind: acmm.FixCreateFile,
			Path: target,
			Body: []byte(body),
		}},
	}, nil
}

// ===== PRTemplate fixer =====

func (PRTemplate) Inputs() []acmm.FixInput { return nil }

func (s PRTemplate) Plan(_ context.Context, idx *scanner.RepoIndex, _ map[string]string) (acmm.FixPlan, error) {
	target := ".github/pull_request_template.md"
	body := `## Summary

<!-- One paragraph: what changed and why. -->

## Test plan

- [ ] Unit tests cover the change
- [ ] Lint and type-check pass
- [ ] Manual smoke check (if UI / behavior changed)
- [ ] Docs updated (if a public surface changed)
- [ ] No unrelated formatting churn in the diff

## Risks / rollback

<!-- What could go wrong; how to revert if it does. -->
`
	// If a template already exists somewhere, suggest appending more
	// checkboxes rather than creating a new file.
	for _, p := range prTemplatePaths {
		if _, err := idx.Read(p); err == nil {
			return acmm.FixPlan{
				SignalID: s.ID(),
				Summary:  fmt.Sprintf("Append checklist items to %s", p),
				Ops: []acmm.FixOp{{
					Kind: acmm.FixAppendFile,
					Path: p,
					Body: []byte("\n<!-- plumbline: appended block -->\n\n## Additional checks\n\n- [ ] Lint and type-check pass\n- [ ] Manual smoke check\n- [ ] Docs updated\n"),
				}},
			}, nil
		}
	}
	return acmm.FixPlan{
		SignalID: s.ID(),
		Summary:  fmt.Sprintf("Create %s", target),
		Ops: []acmm.FixOp{{
			Kind: acmm.FixCreateFile,
			Path: target,
			Body: []byte(body),
		}},
	}, nil
}

// ===== CommitRules fixer =====

func (CommitRules) Inputs() []acmm.FixInput { return nil }

func (s CommitRules) Plan(_ context.Context, _ *scanner.RepoIndex, _ map[string]string) (acmm.FixPlan, error) {
	target := ".gitmessage"
	body := `# subject (≤72 chars): <type>(<scope>): <imperative summary>
#
# Body — wrap at 72 characters. Explain WHY this change is needed and
# WHAT the user-visible effect is. Skip the "what" if the diff is
# self-explanatory.
#
# <type>: feat | fix | docs | refactor | test | chore | perf | ci
#
# After saving, run:
#   git config commit.template .gitmessage
# (the per-repo .git/config is local, so each contributor opts in).
`
	return acmm.FixPlan{
		SignalID: s.ID(),
		Summary:  "Create .gitmessage with a conventional-commit template",
		Ops: []acmm.FixOp{{
			Kind: acmm.FixCreateFile,
			Path: target,
			Body: []byte(body),
		}},
	}, nil
}

// getOr returns inputs[key] if present and non-blank, otherwise fallback.
func getOr(inputs map[string]string, key, fallback string) string {
	v, ok := inputs[key]
	if !ok {
		return fallback
	}
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
