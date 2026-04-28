package l2

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/pkg/acmm"
)

func longBody(lines int) string {
	return strings.Repeat("Some non-trivial line of guidance.\n", lines)
}

func runSignal(t *testing.T, sig signalLike, files fstest.MapFS) acmm.Result {
	t.Helper()
	idx, err := scanner.ScanFS(files, "/repo")
	if err != nil {
		t.Fatalf("ScanFS: %v", err)
	}
	return sig.Detect(context.Background(), idx)
}

// signalLike avoids a circular import on internal/signals.Signal.
type signalLike interface {
	Detect(context.Context, *scanner.RepoIndex) acmm.Result
}

func TestCopilotInstructions(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  float64
	}{
		{"absent", fstest.MapFS{}, acmm.ScoreMissing},
		{"stubbed", fstest.MapFS{".github/copilot-instructions.md": {Data: []byte("just text\nno markdown\n")}}, acmm.ScoreStubbed},
		{"incomplete: heading only", fstest.MapFS{".github/copilot-instructions.md": {Data: []byte("# h\n\nshort\n")}}, acmm.ScoreIncomplete},
		{"found", fstest.MapFS{".github/copilot-instructions.md": {Data: []byte("# Copilot\n\n" + longBody(25))}}, acmm.ScoreFound},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := runSignal(t, CopilotInstructions{}, c.files)
			if got.Score != c.want {
				t.Errorf("score = %v, want %v", got.Score, c.want)
			}
		})
	}
}

func TestContributorGuide(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  float64
	}{
		{"absent", fstest.MapFS{}, acmm.ScoreMissing},
		{"CONTRIBUTING at root, found", fstest.MapFS{"CONTRIBUTING.md": {Data: []byte("# Contributing\n\n" + longBody(25))}}, acmm.ScoreFound},
		{".github/CONTRIBUTING.md found", fstest.MapFS{".github/CONTRIBUTING.md": {Data: []byte("# c\n\n" + longBody(25))}}, acmm.ScoreFound},
		{"stubbed: short, no heading", fstest.MapFS{"CONTRIBUTING.md": {Data: []byte("be nice\n")}}, acmm.ScoreStubbed},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := runSignal(t, ContributorGuide{}, c.files)
			if got.Score != c.want {
				t.Errorf("score = %v, want %v", got.Score, c.want)
			}
		})
	}
}

func TestPRTemplate(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  float64
	}{
		{"absent", fstest.MapFS{}, acmm.ScoreMissing},
		{"stubbed: present, no checkboxes", fstest.MapFS{".github/pull_request_template.md": {Data: []byte("## Summary\n\nText only.\n")}}, acmm.ScoreStubbed},
		{"incomplete: 1 checkbox", fstest.MapFS{".github/pull_request_template.md": {Data: []byte("- [ ] one\n")}}, acmm.ScoreIncomplete},
		{"found: 3 checkboxes", fstest.MapFS{".github/pull_request_template.md": {Data: []byte("- [ ] a\n- [ ] b\n- [ ] c\n")}}, acmm.ScoreFound},
		{"found: PULL_REQUEST_TEMPLATE/ dir", fstest.MapFS{".github/PULL_REQUEST_TEMPLATE/feature.md": {Data: []byte("- [ ] a\n- [ ] b\n- [ ] c\n- [ ] d\n")}}, acmm.ScoreFound},
		{"found: completed checkboxes count", fstest.MapFS{".github/pull_request_template.md": {Data: []byte("- [x] a\n- [X] b\n- [ ] c\n")}}, acmm.ScoreFound},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := runSignal(t, PRTemplate{}, c.files)
			if got.Score != c.want {
				t.Errorf("score = %v, want %v", got.Score, c.want)
			}
		})
	}
}

func TestCommitRules(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  float64
	}{
		{"absent", fstest.MapFS{}, acmm.ScoreMissing},
		{".gitmessage at root", fstest.MapFS{".gitmessage": {Data: []byte("subject")}}, acmm.ScoreFound},
		{".commitlintrc.yml", fstest.MapFS{".commitlintrc.yml": {Data: []byte("extends: ['@commitlint/config-conventional']")}}, acmm.ScoreFound},
		{"commitlint.config.js", fstest.MapFS{"commitlint.config.js": {Data: []byte("module.exports = {extends: ['@commitlint/config-conventional']};")}}, acmm.ScoreFound},
		{"commit-convention.md", fstest.MapFS{".github/commit-convention.md": {Data: []byte("# rules")}}, acmm.ScoreFound},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := runSignal(t, CommitRules{}, c.files)
			if got.Score != c.want {
				t.Errorf("score = %v, want %v", got.Score, c.want)
			}
		})
	}
}
