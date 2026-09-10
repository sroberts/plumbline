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

// unguardedAutonomy opens PRs automatically and has nothing that runs on
// a change. This is the L4 anti-pattern the paper names: a machine
// committing to a codebase nothing is checking.
const unguardedAutonomy = `
name: auto
on:
  schedule:
    - cron: "0 8 * * 1"
jobs:
  bump:
    runs-on: ubuntu-latest
    steps:
      - name: open bump PR
        uses: peter-evans/create-pull-request@v7
`

// guardedAutonomy is the same automation with a PR gate that can fail.
const guardedAutonomy = `
name: CI
on: [pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: test
        run: go test ./...
`

func TestMeasurementBacked(t *testing.T) {
	cases := []struct {
		name       string
		files      fstest.MapFS
		wantStatus acmm.Status
	}{
		{
			"no automation is not gradeable",
			fstest.MapFS{".github/workflows/ci.yml": {Data: []byte(guardedAutonomy)}},
			acmm.StatusNA,
		},
		{
			"automation with no blocking gate",
			fstest.MapFS{".github/workflows/auto.yml": {Data: []byte(unguardedAutonomy)}},
			acmm.StatusMissing,
		},
		{
			"automation plus a PR gate",
			fstest.MapFS{
				".github/workflows/auto.yml": {Data: []byte(unguardedAutonomy)},
				".github/workflows/ci.yml":   {Data: []byte(guardedAutonomy)},
			},
			acmm.StatusFound,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := runOn(t, MeasurementBacked{}, c.files)
			if got.Status != c.wantStatus {
				t.Errorf("status = %v, want %v (notes: %v)", got.Status, c.wantStatus, got.Notes)
			}
		})
	}
}

// A nightly suite reports; it does not block a merge. Counting scheduled
// workflows as guardrails would pass exactly the repos this signal exists
// to catch — automation running against a codebase whose only checks
// happen after the fact.
func TestMeasurementBackedIgnoresScheduledSuites(t *testing.T) {
	files := fstest.MapFS{
		".github/workflows/auto.yml": {Data: []byte(unguardedAutonomy)},
		".github/workflows/nightly.yml": {Data: []byte(`
name: nightly
on:
  schedule:
    - cron: "0 4 * * *"
jobs:
  t:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./...
`)},
	}
	got := runOn(t, MeasurementBacked{}, files)
	if got.Status != acmm.StatusMissing {
		t.Errorf("status = %v, want missing — a nightly suite is not a merge gate", got.Status)
	}
}
