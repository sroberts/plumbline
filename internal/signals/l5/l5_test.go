package l5

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/pkg/acmm"
)

type detector interface {
	Detect(context.Context, *scanner.RepoIndex) acmm.Result
}

func runOn(t *testing.T, d detector, files fstest.MapFS) acmm.Result {
	t.Helper()
	idx, err := scanner.ScanFS(files, "/repo")
	if err != nil {
		t.Fatalf("ScanFS: %v", err)
	}
	return d.Detect(context.Background(), idx)
}

func TestIssueToPR(t *testing.T) {
	files := fstest.MapFS{".github/workflows/issue-pr.yml": {Data: []byte(`
name: issue-to-pr
on:
  issues:
    types: [opened]
jobs:
  j:
    runs-on: ubuntu-latest
    steps:
      - uses: peter-evans/create-pull-request@v5
`)}}
	got := runOn(t, IssueToPR{}, files)
	if got.Score != acmm.ScoreFound {
		t.Errorf("issue-to-pr: score = %v, want Found", got.Score)
	}
}

func TestSelfImprovement(t *testing.T) {
	files := fstest.MapFS{".github/workflows/improve.yml": {Data: []byte(`
name: self-improvement
on:
  pull_request:
    types: [closed]
jobs:
  j:
    runs-on: ubuntu-latest
    steps:
      - run: echo "update CLAUDE.md based on merged PRs"
`)}}
	got := runOn(t, SelfImprovement{}, files)
	if got.Score != acmm.ScoreFound {
		t.Errorf("self-improvement: score = %v, want Found", got.Score)
	}
}

func TestDocsFromPRs(t *testing.T) {
	files := fstest.MapFS{".github/workflows/docs.yml": {Data: []byte(`
name: docs-sync
on: [pull_request]
jobs:
  j:
    runs-on: ubuntu-latest
    steps:
      - run: |
          git commit -m "update docs/"
          git push
        working-directory: docs/
`)}}
	got := runOn(t, DocsFromPRs{}, files)
	if got.Score != acmm.ScoreFound {
		t.Errorf("docs-from-prs: score = %v, want Found", got.Score)
	}
}

func TestMultiRepoOrchestration(t *testing.T) {
	files := fstest.MapFS{".github/workflows/orch.yml": {Data: []byte(`
name: orch
on: [workflow_dispatch]
jobs:
  j:
    runs-on: ubuntu-latest
    steps:
      - run: gh api repos/owner/other-repo/issues
`)}}
	got := runOn(t, MultiRepoOrchestration{}, files)
	if got.Score != acmm.ScoreFound {
		t.Errorf("multi-repo via gh api: score = %v, want Found", got.Score)
	}
}
