package l4

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

func TestSelfModifyingConfig(t *testing.T) {
	files := fstest.MapFS{".github/workflows/auto.yml": {Data: []byte(`
name: a
on: [workflow_dispatch]
jobs:
  j:
    runs-on: ubuntu-latest
    steps:
      - uses: peter-evans/create-pull-request@v5
`)}}
	got := runOn(t, SelfModifyingConfig{}, files)
	if got.Score != acmm.ScoreFound {
		t.Errorf("create-pull-request: score = %v, want Found", got.Score)
	}
}

func TestAutoTriage_SubDailyCron(t *testing.T) {
	// Cron with hour wildcard runs every hour → sub-daily.
	files := fstest.MapFS{".github/workflows/triage.yml": {Data: []byte(`
name: triage
on:
  schedule:
    - cron: "*/15 * * * *"
jobs:
  t: { runs-on: ubuntu-latest, steps: [{ run: "gh issue list" }] }
`)}}
	got := runOn(t, AutoTriage{}, files)
	if got.Score != acmm.ScoreFound {
		t.Errorf("sub-daily issue triage: score = %v, want Found", got.Score)
	}

	// Once-per-day cron should NOT count as auto-triage.
	dailyFiles := fstest.MapFS{".github/workflows/daily.yml": {Data: []byte(`
name: daily
on:
  schedule:
    - cron: "0 5 * * *"
jobs:
  t: { runs-on: ubuntu-latest, steps: [{ run: "gh issue list" }] }
`)}}
	gotDaily := runOn(t, AutoTriage{}, dailyFiles)
	if gotDaily.Score != acmm.ScoreMissing {
		t.Errorf("daily-only schedule: score = %v, want Missing", gotDaily.Score)
	}
}

func TestThresholdBlock(t *testing.T) {
	files := fstest.MapFS{".github/workflows/gate.yml": {Data: []byte(`
name: gate
on: [pull_request]
jobs:
  g:
    runs-on: ubuntu-latest
    steps:
      - if: ${{ fromJson(steps.metrics.outputs.data).rate < 80 }}
        run: exit 1
`)}}
	got := runOn(t, ThresholdBlock{}, files)
	if got.Score != acmm.ScoreFound {
		t.Errorf("threshold conditional: score = %v, want Found", got.Score)
	}
}

func TestWorktreeAgents(t *testing.T) {
	got := runOn(t, WorktreeAgents{}, fstest.MapFS{".devcontainer/devcontainer.json": {Data: []byte("{}")}})
	if got.Score != acmm.ScoreFound {
		t.Errorf(".devcontainer: score = %v, want Found", got.Score)
	}
}

func TestErrorRecovery(t *testing.T) {
	files := fstest.MapFS{".github/workflows/retry.yml": {Data: []byte(`
name: retry
on: [push]
jobs:
  j:
    runs-on: ubuntu-latest
    steps:
      - uses: nick-fields/retry@v3
        with:
          max_attempts: 3
          command: go test ./...
`)}}
	got := runOn(t, ErrorRecovery{}, files)
	if got.Score != acmm.ScoreFound {
		t.Errorf("nick-fields/retry: score = %v, want Found", got.Score)
	}
}
