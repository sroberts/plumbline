package workflows

import (
	"regexp"
	"strings"
	"testing"
)

func TestParse_BasicTriggers(t *testing.T) {
	src := `
name: CI
on:
  push:
    branches: [main]
  pull_request:
  schedule:
    - cron: "0 0 * * *"
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: go test -race ./...
`
	f, err := Parse(".github/workflows/ci.yml", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !f.HasPushTrigger() {
		t.Error("HasPushTrigger = false, want true")
	}
	if !f.HasPullRequestTrigger() {
		t.Error("HasPullRequestTrigger = false, want true")
	}
	if !f.HasScheduledTrigger() {
		t.Error("HasScheduledTrigger = false, want true")
	}
	if got := f.CronEntries(); len(got) != 1 || got[0] != "0 0 * * *" {
		t.Errorf("CronEntries = %v, want [0 0 * * *]", got)
	}
	if !f.UsesAction("actions/checkout") {
		t.Error("UsesAction(actions/checkout) = false, want true")
	}
	if !f.AnyRunMatches(regexp.MustCompile(`go test`)) {
		t.Error(`AnyRunMatches(go test) = false`)
	}
}

func TestParse_OnAsString(t *testing.T) {
	f, err := Parse("a.yml", []byte(`name: x`+"\n"+`on: push`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !f.HasPushTrigger() {
		t.Error("HasPushTrigger = false, want true (on: push as scalar)")
	}
}

func TestParse_OnAsList(t *testing.T) {
	f, err := Parse("a.yml", []byte(`name: x`+"\n"+`on: [push, pull_request]`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !f.HasPushTrigger() || !f.HasPullRequestTrigger() {
		t.Errorf("expected push and pull_request triggers from list form")
	}
}

func TestParse_IssuesTriggerTypes(t *testing.T) {
	src := `
name: triage
on:
  issues:
    types: [opened, labeled]
jobs:
  t: { runs-on: ubuntu-latest, steps: [{ run: "echo" }] }
`
	f, err := Parse("a.yml", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !f.HasIssuesTrigger() {
		t.Fatal("HasIssuesTrigger = false")
	}
	if !f.IssuesTriggerHasType("opened") {
		t.Error("IssuesTriggerHasType(opened) = false")
	}
	if f.IssuesTriggerHasType("closed") {
		t.Error("IssuesTriggerHasType(closed) = true; should be false")
	}
}

func TestParse_PullRequestClosed(t *testing.T) {
	src := `
on:
  pull_request:
    types: [closed]
jobs: {}
`
	f, _ := Parse("a.yml", []byte(src))
	if !f.PullRequestClosed() {
		t.Error("PullRequestClosed = false, want true")
	}
}

func TestParse_RawAccessors(t *testing.T) {
	src := `name: x` + "\n" + `on: push` + "\n" + `jobs: {}`
	f, _ := Parse("a.yml", []byte(src))
	if !f.RawContains("on: push") {
		t.Error("RawContains failed")
	}
	if !strings.Contains(string(f.Raw), "name: x") {
		t.Error("Raw bytes not preserved")
	}
}

func TestParse_BadYAMLReturnsError(t *testing.T) {
	if _, err := Parse("a.yml", []byte("this isn't yaml: ][")); err == nil {
		t.Error("expected parse error")
	}
}

func TestParse_EnvAtEveryScope(t *testing.T) {
	src := `
name: x
on: [pull_request]
env:
  WORKFLOW_LEVEL: "1"
jobs:
  a:
    runs-on: ubuntu-latest
    env:
      JOB_LEVEL: "2"
    steps:
      - name: gate
        env:
          COVERAGE_THRESHOLD: "85.0"
        run: echo hi
`
	f, err := Parse("a.yml", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for _, key := range []string{"WORKFLOW_LEVEL", "JOB_LEVEL", "COVERAGE_THRESHOLD"} {
		if !f.AnyEnvMatches(regexp.MustCompile(key)) {
			t.Errorf("AnyEnvMatches(%s) = false, want true", key)
		}
	}
	// Values are searched too, not only keys.
	if !f.AnyEnvMatches(regexp.MustCompile(`85\.0`)) {
		t.Error("AnyEnvMatches did not search env values")
	}
	if f.Jobs[0].Steps[0].Env["COVERAGE_THRESHOLD"] != "85.0" {
		t.Error("step env not parsed onto the step")
	}
}

func TestParse_StepAndJobNames(t *testing.T) {
	src := `
name: x
on: [push]
jobs:
  quality:
    runs-on: ubuntu-latest
    steps:
      - name: Lint (staticcheck)
        uses: dominikh/staticcheck-action@v1
`
	f, _ := Parse("a.yml", []byte(src))
	if !f.AnyStepNameMatches(regexp.MustCompile(`(?i)lint`)) {
		t.Error("AnyStepNameMatches missed a step name")
	}
	if !f.AnyStepNameMatches(regexp.MustCompile(`quality`)) {
		t.Error("AnyStepNameMatches missed the job ID")
	}
	if f.Jobs[0].ID != "quality" {
		t.Errorf("job ID = %q, want quality", f.Jobs[0].ID)
	}
}

// Jobs come out of a YAML mapping, whose Go iteration order is random.
// Evidence citations name the first matching step, so job order has to be
// stable or the same repo produces different evidence run to run.
func TestParse_JobOrderIsDeterministic(t *testing.T) {
	src := `
name: x
on: [push]
jobs:
  zebra: { runs-on: ubuntu-latest, steps: [{ run: "z" }] }
  alpha: { runs-on: ubuntu-latest, steps: [{ run: "a" }] }
  mike:  { runs-on: ubuntu-latest, steps: [{ run: "m" }] }
`
	for i := 0; i < 20; i++ {
		f, _ := Parse("a.yml", []byte(src))
		var ids []string
		for _, j := range f.Jobs {
			ids = append(ids, j.ID)
		}
		if got := strings.Join(ids, ","); got != "alpha,mike,zebra" {
			t.Fatalf("job order = %q, want alpha,mike,zebra", got)
		}
		if steps := f.AllSteps(); len(steps) != 3 || steps[0].Run != "a" {
			t.Fatalf("AllSteps order unstable: %+v", steps)
		}
	}
}

// A job that calls a reusable workflow has no `steps:` at all. Before
// job-level `uses:` was parsed, such a job looked empty and every
// workflow signal concluded the work wasn't happening.
func TestParse_JobLevelUses(t *testing.T) {
	src := `
name: CI
on: [pull_request]
jobs:
  lint:
    uses: acme/.github/.github/workflows/golangci-lint.yml@main
    with:
      go-version: "1.23"
  test:
    runs-on: ubuntu-latest
    steps:
      - run: go test ./...
`
	f, err := Parse("a.yml", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var lint Job
	for _, j := range f.Jobs {
		if j.ID == "lint" {
			lint = j
		}
	}
	if !lint.IsReusableCall() {
		t.Fatal("lint job not recognized as a reusable-workflow call")
	}
	if lint.With["go-version"] != "1.23" {
		t.Errorf("reusable-workflow inputs not parsed: %v", lint.With)
	}
	if !f.AnyUsesMatches(regexp.MustCompile(`golangci-lint`)) {
		t.Error("AnyUsesMatches missed a job-level uses")
	}
	if !f.UsesAction("acme/.github") {
		t.Error("UsesAction missed a job-level uses")
	}
}
