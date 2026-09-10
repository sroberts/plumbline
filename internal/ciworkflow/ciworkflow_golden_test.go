package ciworkflow

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// -update rewrites the golden files instead of comparing against them.
// Run `go test ./internal/ciworkflow -update` after an intentional change
// to Render, then read the resulting diff before committing it: that diff
// is the whole point of this test.
var update = flag.Bool("update", false, "rewrite the testdata golden files")

// goldenCases pin the workflow plumbline scaffolds into other people's
// repositories. Every case here is a shape that a bug has actually
// reached: the trackedness check the badge gate needs, the shell quoting
// a path with a space needs, the label the drift gate compares against,
// and the gate-capable-but-ungated variant that once previewed a gate it
// did not install.
//
// This is the only artifact plumbline writes that it does not later read
// back — a broken snapshot or badge fails plumbline's own CI, but a
// broken workflow fails silently in someone else's repo, weeks later.
// Reviewing the diff is the substitute for that feedback loop.
var goldenCases = []struct {
	name    string
	variant string
	opts    Options
}{
	{"full-gate3", VariantFull, Options{Variant: VariantFull, FailBelow: 3}},
	{"gate-gate3", VariantGate, Options{Variant: VariantGate, FailBelow: 3}},
	{"badge-only", VariantBadge, Options{Variant: VariantBadge}},
	// A gate-capable variant with no floor renders no gate step at all.
	{"full-nogate", VariantFull, Options{Variant: VariantFull}},
	// A path needing shell quoting, and a non-default label the workflow
	// must pin so the drift gate compares like with like.
	{"badge-custom", VariantBadge, Options{
		Variant:    VariantBadge,
		BadgePath:  "docs/my badge.svg",
		BadgeLabel: "AI readiness",
	}},
}

func TestRenderMatchesGolden(t *testing.T) {
	for _, c := range goldenCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got, err := Render(c.opts)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			path := filepath.Join("testdata", c.name+".yml")

			if *update {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("updated %s", path)
				return
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run `go test ./internal/ciworkflow -update` to create it): %v", err)
			}
			if got != string(want) {
				t.Errorf("rendered workflow no longer matches %s.\n"+
					"If the change is intended, run `go test ./internal/ciworkflow -update` "+
					"and review the diff — it is what a consumer repo's CI will start doing.\n"+
					"--- got ---\n%s\n--- want ---\n%s", path, got, want)
			}
		})
	}
}

// TestGoldenCoversEveryVariant keeps the pin honest as the catalog grows.
// A new variant with no golden file would otherwise ship unreviewed.
func TestGoldenCoversEveryVariant(t *testing.T) {
	covered := map[string]bool{}
	for _, c := range goldenCases {
		covered[c.variant] = true
	}
	for _, v := range Variants() {
		if !covered[v.ID] {
			t.Errorf("variant %q has no golden case; add one to goldenCases so its "+
				"generated workflow is reviewed rather than assumed", v.ID)
		}
	}
}
