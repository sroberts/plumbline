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
