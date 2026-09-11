package dependabot

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sroberts/plumbline/internal/scanner"
	"gopkg.in/yaml.v3"
)

func idxOf(t *testing.T, files fstest.MapFS) *scanner.RepoIndex {
	t.Helper()
	idx, err := scanner.ScanFS(files, "/repo")
	if err != nil {
		t.Fatalf("ScanFS: %v", err)
	}
	return idx
}

// TestDetect_EcosystemsFromManifests — the config has to describe the
// repo it is written into. A static template listing ecosystems the repo
// does not use makes Dependabot log errors for every missing manifest.
func TestDetect_EcosystemsFromManifests(t *testing.T) {
	cases := []struct {
		name  string
		files fstest.MapFS
		want  []string
	}{
		{"go", fstest.MapFS{"go.mod": {Data: []byte("module x\n")}}, []string{"gomod"}},
		{"npm", fstest.MapFS{"package.json": {Data: []byte("{}")}}, []string{"npm"}},
		{"cargo", fstest.MapFS{"Cargo.toml": {Data: []byte("[package]")}}, []string{"cargo"}},
		{"pip-req", fstest.MapFS{"requirements.txt": {Data: []byte("x\n")}}, []string{"pip"}},
		{"pip-pyproject", fstest.MapFS{"pyproject.toml": {Data: []byte("[project]")}}, []string{"pip"}},
		{"docker", fstest.MapFS{"Dockerfile": {Data: []byte("FROM x")}}, []string{"docker"}},
		{"actions", fstest.MapFS{".github/workflows/ci.yml": {Data: []byte("name: CI\non: [push]\njobs: {}\n")}}, []string{"github-actions"}},
		{
			"go plus actions",
			fstest.MapFS{
				"go.mod":                   {Data: []byte("module x\n")},
				".github/workflows/ci.yml": {Data: []byte("name: CI\non: [push]\njobs: {}\n")},
			},
			[]string{"github-actions", "gomod"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Detect(idxOf(t, c.files))
			var ids []string
			for _, e := range got {
				ids = append(ids, e.Ecosystem)
			}
			if strings.Join(ids, ",") != strings.Join(c.want, ",") {
				t.Errorf("Detect = %v, want %v", ids, c.want)
			}
		})
	}
}

// TestDetect_EmptyRepoHasNothingToUpdate — writing a config with no
// update blocks is worse than writing none: it looks configured and does
// nothing.
func TestDetect_EmptyRepoHasNothingToUpdate(t *testing.T) {
	if got := Detect(idxOf(t, fstest.MapFS{"README.md": {Data: []byte("# r")}})); len(got) != 0 {
		t.Errorf("Detect on a repo with no manifests = %v, want none", got)
	}
}

// TestDetect_NestedManifestGetsItsOwnDirectory — Dependabot keys updates
// by directory, so a manifest in a subdirectory needs its own block or it
// is never checked.
func TestDetect_NestedManifestGetsItsOwnDirectory(t *testing.T) {
	files := fstest.MapFS{
		"go.mod":       {Data: []byte("module x\n")},
		"tools/go.mod": {Data: []byte("module x/tools\n")},
	}
	got := Detect(idxOf(t, files))
	var dirs []string
	for _, e := range got {
		if e.Ecosystem == "gomod" {
			dirs = append(dirs, e.Directory)
		}
	}
	if len(dirs) != 2 || dirs[0] != "/" || dirs[1] != "/tools" {
		t.Errorf("gomod directories = %v, want [/ /tools]", dirs)
	}
}

// TestRender_ValidDependabotYAML is the load-bearing check: a malformed
// config makes GitHub silently disable updates for the repo.
func TestRender_ValidDependabotYAML(t *testing.T) {
	files := fstest.MapFS{
		"go.mod":                   {Data: []byte("module x\n")},
		".github/workflows/ci.yml": {Data: []byte("name: CI\non: [push]\njobs: {}\n")},
	}
	body, err := Render(Detect(idxOf(t, files)), Options{})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Version int `yaml:"version"`
		Updates []struct {
			PackageEcosystem string `yaml:"package-ecosystem"`
			Directory        string `yaml:"directory"`
			Schedule         struct {
				Interval string `yaml:"interval"`
			} `yaml:"schedule"`
		} `yaml:"updates"`
	}
	if err := yaml.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("not valid YAML: %v\n%s", err, body)
	}
	if doc.Version != 2 {
		t.Errorf("version = %d, want 2 (the only version GitHub accepts)", doc.Version)
	}
	if len(doc.Updates) != 2 {
		t.Fatalf("updates = %d, want 2", len(doc.Updates))
	}
	for _, u := range doc.Updates {
		if u.PackageEcosystem == "" || u.Directory == "" || u.Schedule.Interval == "" {
			t.Errorf("incomplete update block: %+v", u)
		}
	}
}

// TestRender_IntervalHonored covers the one knob worth exposing.
func TestRender_IntervalHonored(t *testing.T) {
	e := []Update{{Ecosystem: "gomod", Directory: "/"}}
	body, err := Render(e, Options{Interval: "monthly"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "interval: monthly") {
		t.Errorf("interval not honored:\n%s", body)
	}
	if _, err := Render(e, Options{Interval: "hourly"}); err == nil {
		t.Errorf("an interval GitHub does not accept should be rejected")
	}
}

// TestRender_Deterministic — the config is committed, so identical input
// must produce identical bytes.
func TestRender_Deterministic(t *testing.T) {
	e := []Update{{Ecosystem: "gomod", Directory: "/"}, {Ecosystem: "npm", Directory: "/web"}}
	a, _ := Render(e, Options{})
	b, _ := Render(e, Options{})
	if a != b {
		t.Errorf("Render is not deterministic")
	}
}

// TestRender_RefusesEmpty guards the "looks configured, does nothing"
// failure directly.
func TestRender_RefusesEmpty(t *testing.T) {
	if _, err := Render(nil, Options{}); err == nil {
		t.Errorf("rendering with no ecosystems should error rather than emit an empty config")
	}
}

// TestNewPlan_WritesTheCanonicalPath — GitHub reads exactly one location.
func TestNewPlan_WritesTheCanonicalPath(t *testing.T) {
	plan, err := NewPlan([]Update{{Ecosystem: "gomod", Directory: "/"}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Ops) != 1 || plan.Ops[0].Path != DefaultPath {
		t.Fatalf("plan writes %v, want %s", plan.Ops, DefaultPath)
	}
	if string(plan.Ops[0].Kind) != "create-file" {
		t.Errorf("must not clobber an existing dependabot config")
	}
	if !strings.HasPrefix(plan.SignalID, "install-dependabot") {
		t.Errorf("plan id = %q, want an install-dependabot prefix", plan.SignalID)
	}
}
