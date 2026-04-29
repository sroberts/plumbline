package l3

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

const ciWorkflowYAML = `
name: CI
on:
  push:
    branches: [main]
  pull_request:
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: go build ./...
      - run: golangci-lint run
`

const coverageWithThreshold = `
name: cov
on:
  pull_request:
jobs:
  cov:
    runs-on: ubuntu-latest
    steps:
      - run: pytest --cov-fail-under=80
`

const nightlyWorkflow = `
name: nightly-compliance
on:
  schedule:
    - cron: "0 5 * * *"
jobs:
  c:
    runs-on: ubuntu-latest
    steps:
      - run: echo
`

func TestBuildLintGate(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  float64
	}{
		{"missing", fstest.MapFS{}, acmm.ScoreMissing},
		{"both build and lint", fstest.MapFS{".github/workflows/ci.yml": {Data: []byte(ciWorkflowYAML)}}, acmm.ScoreFound},
		{"only build", fstest.MapFS{".github/workflows/build.yml": {Data: []byte(`
name: build
on: [push]
jobs:
  b: { runs-on: ubuntu-latest, steps: [{ run: "go build ./..." }] }
`)}}, acmm.ScoreIncomplete},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := runOn(t, BuildLintGate{}, c.files)
			if got.Score != c.want {
				t.Errorf("score = %v, want %v", got.Score, c.want)
			}
		})
	}
}

func TestCoverageGate(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  float64
	}{
		{"missing", fstest.MapFS{}, acmm.ScoreMissing},
		{"codecov.yml with target", fstest.MapFS{"codecov.yml": {Data: []byte("coverage:\n  status:\n    project:\n      default:\n        target: 80%\n")}}, acmm.ScoreFound},
		{"PR workflow with --cov-fail-under", fstest.MapFS{".github/workflows/cov.yml": {Data: []byte(coverageWithThreshold)}}, acmm.ScoreFound},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := runOn(t, CoverageGate{}, c.files)
			if got.Score != c.want {
				t.Errorf("score = %v, want %v", got.Score, c.want)
			}
		})
	}
}

func TestCoverageSuite(t *testing.T) {
	files := fstest.MapFS{".github/workflows/cov-nightly.yml": {Data: []byte(`
name: cov-nightly
on:
  schedule:
    - cron: "0 4 * * *"
jobs:
  c: { runs-on: ubuntu-latest, steps: [{ run: "go test -coverprofile=cov.out ./..." }] }
`)}}
	got := runOn(t, CoverageSuite{}, files)
	if got.Score != acmm.ScoreFound {
		t.Errorf("with scheduled coverage workflow, score = %v, want %v", got.Score, acmm.ScoreFound)
	}
}

func TestNightlyCompliance(t *testing.T) {
	files := fstest.MapFS{".github/workflows/nightly-compliance.yml": {Data: []byte(nightlyWorkflow)}}
	got := runOn(t, NightlyCompliance{}, files)
	if got.Score != acmm.ScoreFound {
		t.Errorf("nightly-compliance.yml: score = %v, want Found", got.Score)
	}

	missing := runOn(t, NightlyCompliance{}, fstest.MapFS{})
	if missing.Score != acmm.ScoreMissing {
		t.Errorf("missing: score = %v, want Missing", missing.Score)
	}
}

func TestFlakyAnalysis(t *testing.T) {
	gotFile := runOn(t, FlakyAnalysis{}, fstest.MapFS{"flaky-tests.json": {Data: []byte("{}")}})
	if gotFile.Score != acmm.ScoreFound {
		t.Errorf("flaky-tests.json: score = %v, want Found", gotFile.Score)
	}

	gotWorkflow := runOn(t, FlakyAnalysis{}, fstest.MapFS{".github/workflows/flaky-analysis.yml": {Data: []byte(`
name: flaky
on:
  schedule:
    - cron: "0 9 * * 1"
jobs:
  f: { runs-on: ubuntu-latest, steps: [{ run: "echo" }] }
`)}})
	if gotWorkflow.Score != acmm.ScoreFound {
		t.Errorf("flaky workflow: score = %v, want Found", gotWorkflow.Score)
	}
}

func TestErrorMonitoring(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  float64
	}{
		{"missing", fstest.MapFS{"package.json": {Data: []byte(`{"name":"x"}`)}}, acmm.ScoreMissing},
		{"sentry in package.json", fstest.MapFS{"package.json": {Data: []byte(`{"dependencies":{"@sentry/browser":"^7.0.0"}}`)}}, acmm.ScoreFound},
		{"sentry-go in go.mod", fstest.MapFS{"go.mod": {Data: []byte("module x\n\nrequire github.com/getsentry/sentry-go v0.20.0\n")}}, acmm.ScoreFound},
		{"opentelemetry", fstest.MapFS{"go.mod": {Data: []byte("module x\n\nrequire go.opentelemetry.io/otel v1.21.0\n")}}, acmm.ScoreFound},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := runOn(t, ErrorMonitoring{}, c.files)
			if got.Score != c.want {
				t.Errorf("score = %v, want %v", got.Score, c.want)
			}
		})
	}
}

func TestUserFeedback(t *testing.T) {
	gotComponent := runOn(t, UserFeedback{}, fstest.MapFS{"web/src/hooks/useNPSSurvey.ts": {Data: []byte("export const useNPSSurvey = () => {}")}})
	if gotComponent.Score != acmm.ScoreFound {
		t.Errorf("NPS component: score = %v, want Found", gotComponent.Score)
	}

	gotTpl := runOn(t, UserFeedback{}, fstest.MapFS{".github/ISSUE_TEMPLATE/feedback.md": {Data: []byte("---\nname: Feedback\n---\n")}})
	if gotTpl.Score != acmm.ScoreFound {
		t.Errorf("feedback template: score = %v, want Found", gotTpl.Score)
	}
}

func TestAcceptanceTracking(t *testing.T) {
	got := runOn(t, AcceptanceTracking{}, fstest.MapFS{"auto-qa-tuning.json": {Data: []byte("{}")}})
	if got.Score != acmm.ScoreFound {
		t.Errorf("auto-qa-tuning.json: score = %v, want Found", got.Score)
	}
}

func TestUserFeedback_DoesNotSelfDetectOnPlumblineSource(t *testing.T) {
	// Plumbline's own internal/signals/l3/user_feedback.go used to be a
	// false positive. Detection must require a frontend-shaped path
	// in addition to a feedback-flavored filename.
	files := fstest.MapFS{
		"internal/signals/l3/user_feedback.go": {Data: []byte("package l3")},
	}
	got := runOn(t, UserFeedback{}, files)
	if got.Score != acmm.ScoreMissing {
		t.Errorf("self-detection: score = %v, want missing", got.Score)
	}
}

func TestUserFeedback_MatchesGenuineComponent(t *testing.T) {
	files := fstest.MapFS{
		"web/src/hooks/useNPSSurvey.ts": {Data: []byte("export const useNPSSurvey = () => {}")},
	}
	got := runOn(t, UserFeedback{}, files)
	if got.Score != acmm.ScoreFound {
		t.Errorf("genuine NPS component: score = %v, want found", got.Score)
	}
}

func TestUserFeedback_DocFileWithFeedbackInNameIsRejected(t *testing.T) {
	files := fstest.MapFS{
		"docs/feedback-policy.md": {Data: []byte("# how we handle feedback")},
	}
	got := runOn(t, UserFeedback{}, files)
	if got.Score != acmm.ScoreMissing {
		t.Errorf("docs file mentioning feedback: score = %v, want missing", got.Score)
	}
}
