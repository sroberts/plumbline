package l3

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// wf builds a single-workflow repo fixture.
func wf(body string) fstest.MapFS {
	return fstest.MapFS{".github/workflows/ci.yml": {Data: []byte(body)}}
}

// TestBuildLintGate_RemoteGoInstallIsNotABuild — `go install <mod>@<ver>`
// fetches a *remote* tool: a linter, a codegen binary, plumbline itself.
// It compiles nothing belonging to the repo under review, so crediting it
// as a build step says "this repo builds in CI" about a workflow that
// never touches the repo's code.
//
// This was found through plumbline scoring its own scaffolded workflow at
// partial credit. The workflow's only matching line was
// `go install github.com/sroberts/plumbline/cmd/plumbline@latest` — the
// step that installs the assessor. The same false positive fires for any
// repo whose CI installs a Go tool that way.
func TestBuildLintGate_RemoteGoInstallIsNotABuild(t *testing.T) {
	files := wf(`
name: CI
on: [pull_request]
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - run: go install github.com/sroberts/plumbline/cmd/plumbline@latest
      - run: plumbline assess --fail-below 3 --quiet .
`)
	got := runOn(t, BuildLintGate{}, files)
	if got.Score != acmm.ScoreMissing {
		t.Errorf("score = %v, want %v — installing a remote tool is not building this repo (notes: %v)",
			got.Score, acmm.ScoreMissing, got.Notes)
	}
}

// TestBuildLintGate_LocalGoInstallIsABuild keeps the other half true.
// `go install ./cmd/foo` and a bare `go install` compile code in this
// repo, so they remain build evidence.
func TestBuildLintGate_LocalGoInstallIsABuild(t *testing.T) {
	for _, cmd := range []string{"go install ./cmd/plumbline", "go install", "go install ./..."} {
		files := wf(`
name: CI
on: [pull_request]
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - run: ` + cmd + `
`)
		got := runOn(t, BuildLintGate{}, files)
		if got.Score == acmm.ScoreMissing {
			t.Errorf("%q scored missing; it builds this repo's own code", cmd)
		}
	}
}

// TestBuildLintGate_ToolInstallAlongsideRealBuild — a workflow that
// installs a tool *and* builds the repo still counts. Stripping the tool
// fetch must not blind the detector to a real build on the same line.
func TestBuildLintGate_ToolInstallAlongsideRealBuild(t *testing.T) {
	files := wf(`
name: CI
on: [pull_request]
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - run: |
          go install golang.org/x/tools/cmd/goimports@latest
          go build ./...
      - run: golangci-lint run
`)
	got := runOn(t, BuildLintGate{}, files)
	if got.Score != acmm.ScoreFound {
		t.Errorf("score = %v, want %v — the workflow lints and really does build (notes: %v)",
			got.Score, acmm.ScoreFound, got.Notes)
	}
}

// TestBuildLintGate_ScaffoldedWorkflowEarnsNoBuildCredit closes the loop
// SPEC.md §4 opens: plumbline must not credit a repo for gating its own
// build when all the workflow plumbline wrote does is run plumbline.
func TestBuildLintGate_ScaffoldedWorkflowEarnsNoBuildCredit(t *testing.T) {
	body := `
name: ACMM maturity
on:
  push:
    branches: [main]
  pull_request:
permissions:
  contents: read
jobs:
  plumbline:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - name: install plumbline
        run: go install github.com/sroberts/plumbline/cmd/plumbline@latest
      - name: ACMM gate
        run: plumbline assess --fail-below 3 --quiet .
`
	got := runOn(t, BuildLintGate{}, wf(body))
	if got.Score != acmm.ScoreMissing {
		t.Errorf("plumbline's own scaffolded workflow scored %v for build/lint; "+
			"it gates plumbline, not the repo's build (notes: %v, status %v)",
			got.Score, got.Notes, got.Status)
	}
	if strings.Contains(strings.Join(got.Notes, " "), "build step") {
		t.Errorf("notes still claim a build step: %v", got.Notes)
	}
}
