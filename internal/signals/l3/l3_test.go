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

// goToolchainCI is the shape a Go repo that lints with the standard
// toolchain produces: `go vet` plus a staticcheck action, no
// golangci-lint anywhere. Taken from github.com/sroberts/decant, which
// scored 0.67 on both L3 gates while genuinely implementing both.
const goToolchainCI = `
name: CI
on:
  push:
    branches: [main]
  pull_request:
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - name: Build
        run: go build ./...
      - name: Check formatting
        run: |
          unformatted=$(gofmt -l .)
          if [ -n "$unformatted" ]; then
            exit 1
          fi
      - name: Lint
        run: go vet ./...
      - name: Lint (staticcheck)
        uses: dominikh/staticcheck-action@v1
  corpus:
    runs-on: ubuntu-latest
    steps:
      - name: Coverage gate
        env:
          COVERAGE_THRESHOLD: "85.0"
        run: |
          go test -coverpkg=./... -coverprofile=coverage.out ./...
          total=$(go tool cover -func=coverage.out | tail -1 | grep -oE '[0-9]+\.[0-9]+')
          threshold="${COVERAGE_THRESHOLD}"
          awk -v t="$total" -v f="$threshold" 'BEGIN { exit (t+0 >= f+0) ? 0 : 1 }' || {
            echo "coverage ${total}% is below the ${threshold}% floor"
            exit 1
          }
`

// splitCoverageGateCI puts the measurement, the comparison, and the
// failure in three separate steps, with the threshold in a GitHub-native
// if: expression. This is plumbline's own ci.yml shape.
const splitCoverageGateCI = `
name: CI
on:
  push:
    branches: [main]
  pull_request:
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: measure coverage
        id: cov
        run: |
          total=$(go tool cover -func=coverage.txt | awk '/^total:/ {gsub("%",""); print $3}')
          floor=$(cat .coverage-floor 2>/dev/null || echo 60)
          echo "data={\"total\": ${total}, \"floor\": ${floor}}" >> "$GITHUB_OUTPUT"
      - name: coverage hard floor
        if: ${{ fromJson(steps.cov.outputs.data).total < 50 }}
        run: |
          echo "::error::coverage is below the hard floor"
          exit 1
`

// coverageReportedNotGated measures coverage and prints it, but nothing
// fails. The L3 dashboard-graveyard anti-pattern — it must stay partial.
const coverageReportedNotGated = `
name: CI
on:
  pull_request:
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: |
          go test -coverprofile=coverage.out ./...
          go tool cover -func=coverage.out | tail -1
`

// maturityGateCI invokes plumbline's own --fail-below flag. It is a gate,
// but not a coverage gate: the threshold vocabulary must not claim it.
const maturityGateCI = `
name: canary
on:
  pull_request:
jobs:
  canary:
    runs-on: ubuntu-latest
    steps:
      - run: /tmp/plumbline assess --quiet --fail-below 3 .
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
		{"go vet and staticcheck count as lint", fstest.MapFS{".github/workflows/ci.yml": {Data: []byte(goToolchainCI)}}, acmm.ScoreFound},
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
		{"hand-rolled Go gate in one step", fstest.MapFS{".github/workflows/ci.yml": {Data: []byte(goToolchainCI)}}, acmm.ScoreFound},
		{"Go gate split across steps with an if: threshold", fstest.MapFS{".github/workflows/ci.yml": {Data: []byte(splitCoverageGateCI)}}, acmm.ScoreFound},
		{"coverage measured but never gated stays partial", fstest.MapFS{".github/workflows/ci.yml": {Data: []byte(coverageReportedNotGated)}}, acmm.ScoreIncomplete},
		{"non-coverage --fail-below is not a coverage gate", fstest.MapFS{".github/workflows/canary.yml": {Data: []byte(maturityGateCI)}}, acmm.ScoreMissing},
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

// A step named "Lint" whose command we do not recognize still counts,
// because SPEC.md §6 promises job-name matching — but it drops to low
// confidence, since a name is a claim rather than an executed linter.
func TestBuildLintGateNameFallbackIsLowConfidence(t *testing.T) {
	files := fstest.MapFS{".github/workflows/ci.yml": {Data: []byte(`
name: CI
on: [pull_request]
jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - run: go build ./...
      - name: Lint
        run: ./hack/house-style.sh
`)}}
	got := runOn(t, BuildLintGate{}, files)
	if got.Score != acmm.ScoreFound {
		t.Errorf("score = %v, want %v", got.Score, acmm.ScoreFound)
	}
	if got.Confidence != acmm.ConfidenceLow {
		t.Errorf("confidence = %v, want low", got.Confidence)
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

// dashboardGraveyardCI publishes coverage to an external service and does
// nothing else with it. No threshold, no failure — the L3 anti-pattern.
const dashboardGraveyardCI = `
name: CI
on: [pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: go test -coverprofile=coverage.out ./...
      - name: codecov upload
        uses: codecov/codecov-action@v5
        with:
          files: ./coverage.out
`

// binaryUploadOnlyCI uploads build output, not metrics. upload-artifact is
// how binaries and logs move between jobs; that is not a dashboard.
const binaryUploadOnlyCI = `
name: CI
on: [pull_request]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: go build -o bin/app ./cmd/app
      - name: upload binary
        uses: actions/upload-artifact@v4
        with:
          name: app
          path: bin/app
`

func TestMetricsActedOn(t *testing.T) {
	cases := []struct {
		name       string
		files      fstest.MapFS
		wantStatus acmm.Status
	}{
		{
			"no publishers is not gradeable",
			fstest.MapFS{".github/workflows/ci.yml": {Data: []byte(ciWorkflowYAML)}},
			acmm.StatusNA,
		},
		{
			"published and never acted on",
			fstest.MapFS{".github/workflows/ci.yml": {Data: []byte(dashboardGraveyardCI)}},
			acmm.StatusMissing,
		},
		{
			"published and gated",
			fstest.MapFS{
				".github/workflows/ci.yml":  {Data: []byte(dashboardGraveyardCI)},
				".github/workflows/cov.yml": {Data: []byte(coverageWithThreshold)},
			},
			acmm.StatusFound,
		},
		{
			"uploading a binary is not publishing a metric",
			fstest.MapFS{".github/workflows/ci.yml": {Data: []byte(binaryUploadOnlyCI)}},
			acmm.StatusNA,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := runOn(t, MetricsActedOn{}, c.files)
			if got.Status != c.wantStatus {
				t.Errorf("status = %v, want %v (notes: %v)", got.Status, c.wantStatus, got.Notes)
			}
		})
	}
}

// NA must not be scored as a failure: scoring excludes NA from the level
// average, so a repo that publishes nothing is not penalized here.
func TestMetricsActedOnNAIsExcludedFromScoring(t *testing.T) {
	got := runOn(t, MetricsActedOn{}, fstest.MapFS{})
	if got.Status != acmm.StatusNA {
		t.Fatalf("status = %v, want na", got.Status)
	}
	if got.Confidence != acmm.ConfidenceHigh {
		t.Errorf("confidence = %v, want high — absence of publishers is directly observable", got.Confidence)
	}
}

// A bare `exit 1` in an unrelated step is not a metrics consumer. Without
// this guard every repo with a gofmt check passes the signal regardless of
// what happens to the numbers it publishes.
func TestMetricsActedOnIgnoresUnrelatedFailures(t *testing.T) {
	files := fstest.MapFS{".github/workflows/ci.yml": {Data: []byte(`
name: CI
on: [pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: gofmt
        run: |
          test -z "$(gofmt -l .)" || exit 1
      - name: codecov upload
        uses: codecov/codecov-action@v5
`)}}
	got := runOn(t, MetricsActedOn{}, files)
	if got.Status != acmm.StatusMissing {
		t.Errorf("status = %v, want missing — gofmt's exit 1 acts on no metric", got.Status)
	}
}

// Splitting lint into a shared org workflow is a normal layout, and the
// calling job has no steps for the detector to read — only a path.
func TestBuildLintGateSeesReusableLintWorkflow(t *testing.T) {
	files := fstest.MapFS{".github/workflows/ci.yml": {Data: []byte(`
name: CI
on: [pull_request]
jobs:
  lint:
    uses: acme/.github/.github/workflows/golangci-lint.yml@main
  build:
    runs-on: ubuntu-latest
    steps:
      - run: go build ./...
`)}}
	got := runOn(t, BuildLintGate{}, files)
	if got.Score != acmm.ScoreFound {
		t.Errorf("score = %v, want %v (notes: %v)", got.Score, acmm.ScoreFound, got.Notes)
	}
	if got.Confidence != acmm.ConfidenceMedium {
		t.Errorf("confidence = %v, want medium — golangci-lint in the path is a real match, not a name guess", got.Confidence)
	}
}
