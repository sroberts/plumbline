package l2

import (
	"context"
	"fmt"
	"strings"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// L2 fixers. Each L2 detector also implements signals.Fixer. The
// contract: if the target file is missing, scaffold it; if it exists
// but is incomplete, append a TODO block prompting the user to expand
// it. We never overwrite existing prose.

// ===== AgentInstructions fixer =====
//
// Single-tool agent directives. The fixer scaffolds CLAUDE.md by
// default (most common); the user can pick a different filename via
// the "filename" input. If any of the recognized agent files already
// exists, we append to that one instead of creating a new one.

func (AgentInstructions) Inputs() []acmm.FixInput {
	return []acmm.FixInput{
		{
			Key:   "filename",
			Label: "Which agent-instructions file should plumbline create / update?",
			Help: "Default is CLAUDE.md (most common). Other valid choices: " +
				"AGENTS.md, .github/copilot-instructions.md, .cursorrules, " +
				".windsurfrules. Pick whichever matches your team's tooling.",
			Kind:     acmm.FixInputText,
			Required: false,
			Default:  "CLAUDE.md",
		},
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

func (s AgentInstructions) Plan(_ context.Context, idx *scanner.RepoIndex, inputs map[string]string) (acmm.FixPlan, error) {
	// 1. If any recognized agent file already exists, append to it
	//    (don't create a competing one).
	for _, p := range agentInstructionsPaths {
		if _, err := idx.Read(p); err == nil {
			body := buildAgentAppend(p, inputs)
			return acmm.FixPlan{
				SignalID: s.ID(),
				Summary:  fmt.Sprintf("Append a guidance expansion block to %s (existing content untouched)", p),
				Ops: []acmm.FixOp{{
					Kind: acmm.FixAppendFile,
					Path: p,
					Body: []byte(body),
				}},
			}, nil
		}
	}

	// 2. Otherwise create a new file at the requested filename.
	target := strings.TrimSpace(inputs["filename"])
	if target == "" {
		target = "CLAUDE.md"
	}
	if !isRecognizedAgentPath(target) {
		return acmm.FixPlan{}, fmt.Errorf("filename %q is not a recognized agent-instructions path; valid: CLAUDE.md, AGENTS.md, .github/copilot-instructions.md, .cursorrules, .windsurfrules", target)
	}

	body := buildAgentCreate(target, inputs)
	return acmm.FixPlan{
		SignalID: s.ID(),
		Summary:  fmt.Sprintf("Create %s with a structured AI guidance template", target),
		Ops: []acmm.FixOp{{
			Kind: acmm.FixCreateFile,
			Path: target,
			Body: []byte(body),
		}},
	}, nil
}

func isRecognizedAgentPath(p string) bool {
	for _, known := range agentInstructionsPaths {
		if p == known {
			return true
		}
	}
	return false
}

func buildAgentCreate(filename string, inputs map[string]string) string {
	summary := getOr(inputs, "project_summary", "TODO: replace this with a one-paragraph project summary.")
	conventions := getOr(inputs, "conventions", "TODO: list the conventions an AI agent should follow.")
	antipatterns := getOr(inputs, "antipatterns", "TODO: list the things AI agents should never do.")
	return fmt.Sprintf(`# %s

This file provides guidance to AI coding agents working in this repository.

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
`, filename, summary, conventions, antipatterns)
}

func buildAgentAppend(path string, inputs map[string]string) string {
	conventions := getOr(inputs, "conventions", "TODO: add the conventions an AI agent should follow.")
	antipatterns := getOr(inputs, "antipatterns", "TODO: add the things AI agents should never do.")
	return fmt.Sprintf(`<!-- plumbline: appended block; expand %s to reach the L2 substantive bar -->

## Additional conventions

%s

## Additional anti-patterns (do NOT)

%s
`, path, conventions, antipatterns)
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
