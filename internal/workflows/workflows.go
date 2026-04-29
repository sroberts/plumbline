// Package workflows parses CI workflow files (GitHub Actions only in
// MVP — see SPEC.md §6 CI-system scope) into a CI-agnostic AST that
// signal detectors consume. Workflow signals MUST go through this AST
// rather than re-parsing raw YAML themselves.
package workflows

import (
	"bytes"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// File is the parsed view of one CI workflow YAML file.
type File struct {
	Path string
	Name string
	On   Triggers
	Jobs []Job
	Raw  []byte // original bytes, for raw-substring/regex fallbacks
}

// Triggers captures the `on:` block. GitHub Actions allows three shapes
// (string / list / map); the parser normalizes them all into this.
type Triggers struct {
	Push              *PushPullTrigger
	PullRequest       *PushPullTrigger
	PullRequestTarget *PushPullTrigger
	Schedule          []ScheduleTrigger
	Issues            *EventTrigger
	WorkflowDispatch  bool
	WorkflowRun       *WorkflowRunTrigger
	PullRequestReview *EventTrigger
}

// PushPullTrigger is the shared shape for push and pull_request.
type PushPullTrigger struct {
	Branches []string
	Paths    []string
	Tags     []string
	Types    []string
}

// ScheduleTrigger is one entry in `on.schedule`.
type ScheduleTrigger struct {
	Cron string
}

// EventTrigger is the generic types-based form (issues, pull_request_review, etc.).
type EventTrigger struct {
	Types []string
}

// WorkflowRunTrigger is `on.workflow_run`.
type WorkflowRunTrigger struct {
	Workflows []string
	Types     []string
}

// Job is one entry under `jobs:`.
type Job struct {
	ID          string
	Name        string
	RunsOn      string
	If          string
	Steps       []Step
	Permissions map[string]string
}

// Step is one entry in `steps:`.
type Step struct {
	Name string
	Uses string
	Run  string
	With map[string]string
	If   string
}

// Parse parses one workflow YAML file. Path is recorded in File.Path
// for evidence-citation purposes; data is the raw YAML bytes.
func Parse(path string, data []byte) (*File, error) {
	var raw rawFile
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	f := &File{Path: path, Name: raw.Name, Raw: data}
	f.On = raw.On.parse()
	for _, jr := range raw.Jobs {
		f.Jobs = append(f.Jobs, jr.toJob())
	}
	return f, nil
}

// HasScheduledTrigger reports whether the workflow has any cron entry.
func (f *File) HasScheduledTrigger() bool { return len(f.On.Schedule) > 0 }

// HasPushTrigger reports whether the workflow runs on push.
func (f *File) HasPushTrigger() bool { return f.On.Push != nil }

// HasPullRequestTrigger reports whether the workflow runs on pull_request.
func (f *File) HasPullRequestTrigger() bool { return f.On.PullRequest != nil }

// HasIssuesTrigger reports whether the workflow runs on issues events.
func (f *File) HasIssuesTrigger() bool { return f.On.Issues != nil }

// IssuesTriggerHasType reports whether the issues trigger fires for the
// given event type (e.g., "opened", "labeled").
func (f *File) IssuesTriggerHasType(t string) bool {
	if f.On.Issues == nil {
		return false
	}
	for _, x := range f.On.Issues.Types {
		if x == t {
			return true
		}
	}
	// No `types:` filter means all types fire.
	return len(f.On.Issues.Types) == 0
}

// PullRequestClosed reports whether the workflow's pull_request trigger
// includes the "closed" event type.
func (f *File) PullRequestClosed() bool {
	if f.On.PullRequest == nil {
		return false
	}
	for _, t := range f.On.PullRequest.Types {
		if t == "closed" {
			return true
		}
	}
	return false
}

// CronEntries returns all cron expressions on the workflow.
func (f *File) CronEntries() []string {
	out := make([]string, 0, len(f.On.Schedule))
	for _, s := range f.On.Schedule {
		out = append(out, s.Cron)
	}
	return out
}

// UsesAction reports whether any step in any job uses an action whose
// `uses:` starts with the given prefix (e.g. "peter-evans/create-pull-request").
func (f *File) UsesAction(prefix string) bool {
	for _, j := range f.Jobs {
		for _, s := range j.Steps {
			if strings.HasPrefix(s.Uses, prefix) {
				return true
			}
		}
	}
	return false
}

// AnyRunMatches reports whether any step's `run:` body matches re.
func (f *File) AnyRunMatches(re *regexp.Regexp) bool {
	for _, j := range f.Jobs {
		for _, s := range j.Steps {
			if s.Run != "" && re.MatchString(s.Run) {
				return true
			}
		}
	}
	return false
}

// RawContains reports whether the raw YAML contains the given substring.
// Useful as a fallback when a signal needs to detect something not in
// the structured AST yet.
func (f *File) RawContains(substr string) bool {
	return bytes.Contains(f.Raw, []byte(substr))
}

// rawFile / rawOn / rawJob mirror the YAML shape with yaml.Node-aware
// fields so we can normalize the various "on:" shapes.
type rawFile struct {
	Name string            `yaml:"name"`
	On   rawOn             `yaml:"on"`
	Jobs map[string]rawJob `yaml:"jobs"`
}

type rawOn struct {
	node yaml.Node
}

func (r *rawOn) UnmarshalYAML(node *yaml.Node) error {
	r.node = *node
	return nil
}

func (r *rawOn) parse() Triggers {
	var t Triggers
	switch r.node.Kind {
	case yaml.ScalarNode:
		// `on: push`
		setSimpleTrigger(&t, r.node.Value)
	case yaml.SequenceNode:
		// `on: [push, pull_request]`
		for _, child := range r.node.Content {
			if child.Kind == yaml.ScalarNode {
				setSimpleTrigger(&t, child.Value)
			}
		}
	case yaml.MappingNode:
		// `on: { push: { branches: [main] }, pull_request: ... }`
		for i := 0; i+1 < len(r.node.Content); i += 2 {
			key := r.node.Content[i].Value
			val := r.node.Content[i+1]
			parseTriggerNode(&t, key, val)
		}
	}
	return t
}

func setSimpleTrigger(t *Triggers, name string) {
	switch name {
	case "push":
		t.Push = &PushPullTrigger{}
	case "pull_request":
		t.PullRequest = &PushPullTrigger{}
	case "pull_request_target":
		t.PullRequestTarget = &PushPullTrigger{}
	case "issues":
		t.Issues = &EventTrigger{}
	case "workflow_dispatch":
		t.WorkflowDispatch = true
	case "pull_request_review":
		t.PullRequestReview = &EventTrigger{}
	}
}

func parseTriggerNode(t *Triggers, key string, val *yaml.Node) {
	switch key {
	case "push":
		t.Push = parsePushPull(val)
	case "pull_request":
		t.PullRequest = parsePushPull(val)
	case "pull_request_target":
		t.PullRequestTarget = parsePushPull(val)
	case "issues":
		t.Issues = parseEventTrigger(val)
	case "pull_request_review":
		t.PullRequestReview = parseEventTrigger(val)
	case "schedule":
		t.Schedule = parseSchedule(val)
	case "workflow_dispatch":
		t.WorkflowDispatch = true
	case "workflow_run":
		t.WorkflowRun = parseWorkflowRun(val)
	}
}

func parsePushPull(n *yaml.Node) *PushPullTrigger {
	if n == nil || n.Kind == yaml.ScalarNode {
		return &PushPullTrigger{}
	}
	if n.Kind != yaml.MappingNode {
		return &PushPullTrigger{}
	}
	t := &PushPullTrigger{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i].Value
		val := n.Content[i+1]
		switch key {
		case "branches":
			t.Branches = scalarList(val)
		case "tags":
			t.Tags = scalarList(val)
		case "paths":
			t.Paths = scalarList(val)
		case "types":
			t.Types = scalarList(val)
		}
	}
	return t
}

func parseEventTrigger(n *yaml.Node) *EventTrigger {
	if n == nil || n.Kind == yaml.ScalarNode {
		return &EventTrigger{}
	}
	t := &EventTrigger{}
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == "types" {
				t.Types = scalarList(n.Content[i+1])
			}
		}
	}
	return t
}

func parseSchedule(n *yaml.Node) []ScheduleTrigger {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	var out []ScheduleTrigger
	for _, child := range n.Content {
		if child.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(child.Content); i += 2 {
			if child.Content[i].Value == "cron" {
				out = append(out, ScheduleTrigger{Cron: child.Content[i+1].Value})
			}
		}
	}
	return out
}

func parseWorkflowRun(n *yaml.Node) *WorkflowRunTrigger {
	if n == nil || n.Kind != yaml.MappingNode {
		return &WorkflowRunTrigger{}
	}
	t := &WorkflowRunTrigger{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i].Value
		val := n.Content[i+1]
		switch key {
		case "workflows":
			t.Workflows = scalarList(val)
		case "types":
			t.Types = scalarList(val)
		}
	}
	return t
}

func scalarList(n *yaml.Node) []string {
	if n == nil {
		return nil
	}
	var out []string
	switch n.Kind {
	case yaml.ScalarNode:
		out = append(out, n.Value)
	case yaml.SequenceNode:
		for _, c := range n.Content {
			if c.Kind == yaml.ScalarNode {
				out = append(out, c.Value)
			}
		}
	}
	return out
}

type rawJob struct {
	Name        string            `yaml:"name"`
	RunsOn      yaml.Node         `yaml:"runs-on"`
	If          string            `yaml:"if"`
	Steps       []rawStep         `yaml:"steps"`
	Permissions map[string]string `yaml:"permissions"`
}

func (r rawJob) toJob() Job {
	runsOn := ""
	switch r.RunsOn.Kind {
	case yaml.ScalarNode:
		runsOn = r.RunsOn.Value
	case yaml.SequenceNode:
		if len(r.RunsOn.Content) > 0 {
			runsOn = r.RunsOn.Content[0].Value
		}
	}
	steps := make([]Step, 0, len(r.Steps))
	for _, s := range r.Steps {
		steps = append(steps, s.toStep())
	}
	return Job{
		Name:        r.Name,
		RunsOn:      runsOn,
		If:          r.If,
		Steps:       steps,
		Permissions: r.Permissions,
	}
}

type rawStep struct {
	Name string            `yaml:"name"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	With map[string]string `yaml:"with"`
	If   string            `yaml:"if"`
}

func (r rawStep) toStep() Step {
	return Step(r)
}
