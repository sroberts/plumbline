package ciworkflow

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestRender_ValidYAML is the load-bearing check: a workflow GitHub
// cannot parse is worse than no workflow — it fails silently on push
// with a repository-level error the author never sees locally.
func TestRender_ValidYAML(t *testing.T) {
	for _, v := range Variants() {
		body, err := Render(Options{Variant: v.ID, FailBelow: 3})
		if err != nil {
			t.Fatalf("%s: %v", v.ID, err)
		}
		var doc struct {
			Name string `yaml:"name"`
			Jobs map[string]struct {
				RunsOn string `yaml:"runs-on"`
				Steps  []struct {
					Uses string `yaml:"uses"`
					Run  string `yaml:"run"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
			Permissions map[string]string `yaml:"permissions"`
		}
		if err := yaml.Unmarshal([]byte(body), &doc); err != nil {
			t.Fatalf("%s: workflow is not valid YAML: %v\n%s", v.ID, err, body)
		}
		if doc.Name == "" {
			t.Errorf("%s: workflow has no name", v.ID)
		}
		job, ok := doc.Jobs["plumbline"]
		if !ok {
			t.Fatalf("%s: no 'plumbline' job:\n%s", v.ID, body)
		}
		if job.RunsOn == "" {
			t.Errorf("%s: job has no runs-on", v.ID)
		}
		if len(job.Steps) < 2 {
			t.Errorf("%s: job has %d steps, expected checkout + at least one action", v.ID, len(job.Steps))
		}
		if doc.Permissions["contents"] != "read" {
			t.Errorf("%s: workflow should request least-privilege contents: read, got %v", v.ID, doc.Permissions)
		}
		if !strings.Contains(body, "actions/checkout") {
			t.Errorf("%s: workflow never checks out the repo", v.ID)
		}
	}
}

// TestRender_VariantContent pins what each variant is actually for.
func TestRender_VariantContent(t *testing.T) {
	gate, err := Render(Options{Variant: VariantGate, FailBelow: 3})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gate, "--fail-below 3") {
		t.Errorf("gate variant missing the gate:\n%s", gate)
	}
	if strings.Contains(gate, "badge") {
		t.Errorf("gate variant should not mention the badge:\n%s", gate)
	}

	badge, err := Render(Options{Variant: VariantBadge})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(badge, "plumbline badge") {
		t.Errorf("badge variant does not generate a badge:\n%s", badge)
	}
	if !strings.Contains(badge, "git diff --exit-code") {
		t.Errorf("badge variant has no drift gate:\n%s", badge)
	}
	if strings.Contains(badge, "--fail-below") {
		t.Errorf("badge variant should not add a level gate:\n%s", badge)
	}

	full, err := Render(Options{Variant: VariantFull, FailBelow: 2})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--fail-below 2", "plumbline badge", "git diff --exit-code"} {
		if !strings.Contains(full, want) {
			t.Errorf("full variant missing %q:\n%s", want, full)
		}
	}
}

// TestRender_FailBelowZeroOmitsGate — a repo not yet at the level it
// wants still benefits from the badge; forcing a gate would make the
// first install red and get the workflow deleted.
func TestRender_FailBelowZeroOmitsGate(t *testing.T) {
	body, err := Render(Options{Variant: VariantFull, FailBelow: 0})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "--fail-below") {
		t.Errorf("FailBelow 0 should emit no gate:\n%s", body)
	}
}

// TestRender_BadgePathHonored lets a repo keep its badge somewhere
// other than the repo root dotfile.
func TestRender_BadgePathHonored(t *testing.T) {
	body, err := Render(Options{Variant: VariantBadge, BadgePath: "docs/acmm.svg"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "docs/acmm.svg") {
		t.Errorf("custom badge path ignored:\n%s", body)
	}
}

// TestRender_Rejects checks the input validation surface.
func TestRender_Rejects(t *testing.T) {
	if _, err := Render(Options{Variant: "nope"}); err == nil {
		t.Errorf("unknown variant should error")
	}
	for _, bad := range []int{1, 6, -1} {
		if _, err := Render(Options{Variant: VariantGate, FailBelow: bad}); err == nil {
			t.Errorf("FailBelow %d should error (valid: 0, or 2-5)", bad)
		}
	}
}

// TestRender_Deterministic — the workflow is a committed artifact, so
// two renders of the same options must match byte for byte.
func TestRender_Deterministic(t *testing.T) {
	a, _ := Render(Options{Variant: VariantFull, FailBelow: 3})
	b, _ := Render(Options{Variant: VariantFull, FailBelow: 3})
	if a != b {
		t.Errorf("Render is not deterministic")
	}
}

// TestNewPlan_WritesWorkflowFile checks the FixPlan the CLI and TUI
// both consume.
func TestNewPlan_WritesWorkflowFile(t *testing.T) {
	plan, err := NewPlan(Options{Variant: VariantFull, FailBelow: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ops) != 1 {
		t.Fatalf("expected exactly one op, got %d", len(plan.Ops))
	}
	op := plan.Ops[0]
	if op.Path != DefaultPath {
		t.Errorf("op path = %q, want %q", op.Path, DefaultPath)
	}
	if string(op.Kind) != "create-file" {
		t.Errorf("op kind = %q, want create-file (installing must never clobber an existing workflow)", op.Kind)
	}
	if plan.Summary == "" {
		t.Errorf("plan has no summary; the TUI preview renders it")
	}
	if !strings.HasPrefix(plan.SignalID, "install-ci:") {
		t.Errorf("plan id = %q, want an install-ci: prefix so the TUI can route it", plan.SignalID)
	}
}

// TestNewPlan_CustomPath supports repos that keep workflows elsewhere
// or want a second, differently-named workflow.
func TestNewPlan_CustomPath(t *testing.T) {
	plan, err := NewPlan(Options{Variant: VariantGate, FailBelow: 3, Path: ".github/workflows/acmm.yml"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Ops[0].Path != ".github/workflows/acmm.yml" {
		t.Errorf("custom path ignored: %q", plan.Ops[0].Path)
	}
}

// TestVariants_Registry backs the CLI --list output and the TUI picker.
func TestVariants_Registry(t *testing.T) {
	vs := Variants()
	if len(vs) < 3 {
		t.Fatalf("expected at least 3 variants, got %d", len(vs))
	}
	for _, v := range vs {
		if v.ID == "" || v.Name == "" || v.Desc == "" {
			t.Errorf("variant %+v has an empty field; the picker renders all three", v)
		}
		got, ok := VariantByID(v.ID)
		if !ok || got.ID != v.ID {
			t.Errorf("VariantByID(%q) did not round-trip", v.ID)
		}
	}
	if _, ok := VariantByID("nope"); ok {
		t.Errorf("VariantByID should reject an unknown id")
	}
}
