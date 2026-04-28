package l2

import (
	"context"
	"errors"
	"io/fs"
	"strings"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// commitRulesPaths is a list of canonical paths a commit-style guide
// or commit-lint config might live at.
var commitRulesPaths = []string{
	".gitmessage",
	".github/commit-convention.md",
	"docs/commit-convention.md",
	"COMMIT_CONVENTION.md",
}

// commitRulesPrefixes match commitlint config files of any flavor
// (.commitlintrc, commitlint.config.js, commitlint.config.cjs, etc.).
var commitRulesPrefixes = []string{
	".commitlintrc",
	"commitlint.config.",
}

// CommitRules detects whether the repo encodes commit-message
// conventions — either as a documented guide or as commitlint config.
// The detection is binary at this point: presence is Found, absence is
// Missing. A future revision could grade depth (e.g., commitlint with
// extends rules vs. a stub config).
type CommitRules struct{}

func (CommitRules) ID() string        { return "l2.commit-rules" }
func (CommitRules) Level() acmm.Level { return acmm.LevelInstructed }
func (CommitRules) Family() string    { return "templates" }
func (CommitRules) Title() string     { return "Commit-message conventions encoded in repo" }

func (s CommitRules) Detect(_ context.Context, idx *scanner.RepoIndex) acmm.Result {
	for _, p := range commitRulesPaths {
		if _, err := idx.Read(p); err == nil {
			return acmm.Result{
				Status:     acmm.StatusFound,
				Score:      acmm.ScoreFound,
				Confidence: acmm.ConfidenceHigh,
				Method:     acmm.MethodFilenameMatch,
				Evidence:   []acmm.Evidence{{Path: p}},
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			// Other errors fall through to next candidate.
			continue
		}
	}

	// Look for any commitlint-style config by basename prefix.
	for base, paths := range idx.ByName {
		for _, prefix := range commitRulesPrefixes {
			if strings.HasPrefix(base, prefix) {
				return acmm.Result{
					Status:     acmm.StatusFound,
					Score:      acmm.ScoreFound,
					Confidence: acmm.ConfidenceHigh,
					Method:     acmm.MethodFilenameMatch,
					Evidence:   []acmm.Evidence{{Path: paths[0]}},
				}
			}
		}
	}

	return acmm.Result{
		Status:     acmm.StatusMissing,
		Score:      acmm.ScoreMissing,
		Confidence: acmm.ConfidenceHigh,
		Method:     acmm.MethodFilenameMatch,
	}
}

func init() {
	signals.Default.Register(CommitRules{})
}
