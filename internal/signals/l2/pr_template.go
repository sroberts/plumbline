package l2

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// prTemplatePaths are PR template locations GitHub recognizes. The
// PULL_REQUEST_TEMPLATE/ directory variant is matched by basename.
var prTemplatePaths = []string{
	".github/pull_request_template.md",
	".github/PULL_REQUEST_TEMPLATE.md",
	"PULL_REQUEST_TEMPLATE.md",
	"docs/pull_request_template.md",
}

const (
	prTemplateMinCheckboxes  = 3
	prTemplateSomeCheckboxes = 1
)

// PRTemplate detects whether the repo has a pull-request template with
// at least three markdown checklist items. A template without checklist
// items is a stub; a missing template is L2's biggest near-miss.
type PRTemplate struct{}

func (PRTemplate) ID() string        { return "l2.pr-template" }
func (PRTemplate) Level() acmm.Level { return acmm.LevelInstructed }
func (PRTemplate) Family() string    { return "templates" }
func (PRTemplate) Title() string     { return "PR template with structured checklist" }

func (s PRTemplate) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	// Try the canonical paths first.
	for _, p := range prTemplatePaths {
		data, err := idx.Read(p)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			continue
		}
		return scorePRTemplate(p, data)
	}

	// Fall back to PULL_REQUEST_TEMPLATE/ dir (multi-template repos).
	for base, paths := range idx.ByName {
		_ = base
		for _, p := range paths {
			if !startsWith(p, ".github/PULL_REQUEST_TEMPLATE/") &&
				!startsWith(p, "PULL_REQUEST_TEMPLATE/") {
				continue
			}
			data, err := idx.Read(p)
			if err != nil {
				continue
			}
			return scorePRTemplate(p, data)
		}
	}

	return acmm.Result{
		Status:     acmm.StatusMissing,
		Score:      acmm.ScoreMissing,
		Confidence: acmm.ConfidenceHigh,
		Method:     acmm.MethodFilenameMatch,
		Notes:      []string{"no PR template found at .github/pull_request_template.md or PULL_REQUEST_TEMPLATE/"},
		FixHint: "Add .github/pull_request_template.md with at least 3 markdown " +
			"checkboxes ('- [ ] item') so AI agents fill in a structured " +
			"checklist on every PR.",
	}
}

func scorePRTemplate(path string, data []byte) acmm.Result {
	checkboxes := countMarkdownCheckboxes(data)
	score := acmm.ScoreStubbed
	switch {
	case checkboxes >= prTemplateMinCheckboxes:
		score = acmm.ScoreFound
	case checkboxes >= prTemplateSomeCheckboxes:
		score = acmm.ScoreIncomplete
	}
	res := acmm.Result{
		Status:     acmm.StatusFromScore(score),
		Score:      score,
		Confidence: acmm.ConfidenceMedium,
		Method:     acmm.MethodContentRegex,
		Evidence: []acmm.Evidence{{
			Path:    path,
			Excerpt: excerpt(data, 160),
		}},
	}
	switch {
	case score == acmm.ScoreFound:
		res.Notes = []string{fmt.Sprintf("%d markdown checkboxes (≥%d)", checkboxes, prTemplateMinCheckboxes)}
	case score == acmm.ScoreIncomplete:
		res.Notes = []string{fmt.Sprintf("only %d markdown checkboxes (need ≥%d for Found)", checkboxes, prTemplateMinCheckboxes)}
		res.FixHint = fmt.Sprintf("Add %d more markdown checkbox(es) to the PR template covering pre-merge checks (tests, docs, version bumps, etc.).",
			prTemplateMinCheckboxes-checkboxes)
	default:
		res.Notes = []string{"PR template exists but has no markdown checkboxes"}
		res.FixHint = "Add at least 3 markdown checkboxes ('- [ ] tests added', etc.) to make the template actionable for AI agents."
	}
	return res
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func init() {
	signals.Default.Register(PRTemplate{})
}
