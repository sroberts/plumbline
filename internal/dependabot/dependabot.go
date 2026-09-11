// Package dependabot renders a .github/dependabot.yml describing the
// dependency ecosystems a repository actually uses, and the FixPlan that
// installs it.
//
// Scope note (SPEC.md §4). plumbline deliberately does not scaffold the
// artifacts its own signals look for — a generator and a detector sharing
// an author agree by construction, and the catalog stops meeting the
// shapes it has not seen.
//
// This package stays outside that rule by writing only the *open* half of
// the loop: update blocks and a schedule, and nothing that merges. In
// ACMM terms Dependabot alone leaves a human in the path (upstream
// release, PR, someone merges); auto-merge gated on CI is what closes it,
// and the closed loop is the L4-shaped one. Keep it that way. Emitting an
// auto-merge workflow here would hand plumbline a file its own catalog
// could credit, which is exactly what §4 forbids.
//
// Today no signal detects the config at all, so running the command moves
// no verdict — but that is a fact about the catalog, not a guarantee.
// The guarantee is the line above: generate the open loop, never the
// closed one.
//
// The content is derived from the repo rather than templated: a config
// listing ecosystems whose manifests are absent makes Dependabot log an
// error for each one, and a config that omits a manifest silently never
// checks it.
package dependabot

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// DefaultPath is the only location GitHub reads a Dependabot config from.
const DefaultPath = ".github/dependabot.yml"

// DefaultInterval is the update cadence used when none is given. Weekly
// is the middle option: daily generates enough PRs on a busy dependency
// tree to be ignored, which is the failure mode that matters.
const DefaultInterval = "weekly"

// validIntervals are the cadences GitHub accepts.
var validIntervals = map[string]bool{"daily": true, "weekly": true, "monthly": true}

// Update is one `updates:` entry — an ecosystem plus the directory whose
// manifest it governs.
type Update struct {
	Ecosystem string
	Directory string
}

// Options tune the rendered config.
type Options struct {
	// Interval is the schedule cadence. Empty means DefaultInterval.
	Interval string
}

func (o Options) interval() string {
	if o.Interval == "" {
		return DefaultInterval
	}
	return o.Interval
}

// manifests maps a manifest filename to the Dependabot ecosystem that
// reads it. Only ecosystems identifiable from a file present in the tree
// are listed: guessing produces a config that errors on every run.
var manifests = []struct {
	file      string
	ecosystem string
}{
	{"go.mod", "gomod"},
	{"package.json", "npm"},
	{"Cargo.toml", "cargo"},
	{"requirements.txt", "pip"},
	{"pyproject.toml", "pip"},
	{"Pipfile", "pip"},
	{"Gemfile", "bundler"},
	{"composer.json", "composer"},
	{"pom.xml", "maven"},
	{"build.gradle", "gradle"},
	{"Dockerfile", "docker"},
	{"*.tf", "terraform"},
}

// Detect returns the update entries a repository warrants, ordered
// deterministically by ecosystem then directory.
//
// GitHub Actions is included whenever the repo has workflows: the action
// versions pinned in them go stale exactly like any other dependency, and
// that is the case most repos miss — a runtime deprecation takes every
// workflow out at once, with no manifest anywhere to hint at it.
func Detect(idx *scanner.RepoIndex) []Update {
	if idx == nil {
		return nil
	}
	seen := map[Update]bool{}
	var out []Update
	add := func(u Update) {
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}

	for _, m := range manifests {
		for _, p := range idx.ByName[m.file] {
			add(Update{Ecosystem: m.ecosystem, Directory: dirOf(p)})
		}
	}
	// *.tf is a pattern rather than a name, so it needs its own pass.
	for _, f := range idx.Files {
		if strings.HasSuffix(f.Path, ".tf") {
			add(Update{Ecosystem: "terraform", Directory: dirOf(f.Path)})
		}
	}
	if len(idx.Workflows) > 0 {
		add(Update{Ecosystem: "github-actions", Directory: "/"})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Ecosystem != out[j].Ecosystem {
			return out[i].Ecosystem < out[j].Ecosystem
		}
		return out[i].Directory < out[j].Directory
	})
	return out
}

// dirOf returns the Dependabot `directory:` value for a manifest path —
// repo-absolute, with "/" for the root.
func dirOf(p string) string {
	d := path.Dir(p)
	if d == "." || d == "" {
		return "/"
	}
	return "/" + d
}

// Render returns the dependabot.yml body for the given updates.
func Render(updates []Update, opts Options) (string, error) {
	if len(updates) == 0 {
		// A config with no update blocks reads as configured and does
		// nothing — strictly worse than having no file, because it stops
		// anyone from noticing the gap.
		return "", fmt.Errorf("no dependency manifests found, so there is nothing for Dependabot to update")
	}
	iv := opts.interval()
	if !validIntervals[iv] {
		return "", fmt.Errorf("invalid interval %q (want daily|weekly|monthly)", iv)
	}

	var b bytes.Buffer
	b.WriteString("# Managed by `plumbline install-dependabot`. Safe to edit by hand.\n")
	b.WriteString("#\n")
	b.WriteString("# One block per dependency manifest found in this repo. Adding an\n")
	b.WriteString("# ecosystem whose manifest is absent makes Dependabot error on every\n")
	b.WriteString("# run; omitting one present means it is never checked.\n")
	b.WriteString("version: 2\n")
	b.WriteString("updates:\n")
	for _, u := range updates {
		fmt.Fprintf(&b, "  - package-ecosystem: %q\n", u.Ecosystem)
		fmt.Fprintf(&b, "    directory: %q\n", u.Directory)
		b.WriteString("    schedule:\n")
		fmt.Fprintf(&b, "      interval: %s\n", iv)
		if u.Ecosystem == "github-actions" {
			b.WriteString("    # Action versions go stale like any dependency, and a runtime\n")
			b.WriteString("    # deprecation takes every workflow out at once — with no\n")
			b.WriteString("    # manifest anywhere to warn you first.\n")
		}
	}
	return b.String(), nil
}

// NewPlan returns the FixPlan that installs the config. The op is a
// create-file: an existing Dependabot config encodes deliberate choices
// (ignores, groups, reviewers) that are not ours to replace.
func NewPlan(updates []Update, opts Options) (acmm.FixPlan, error) {
	body, err := Render(updates, opts)
	if err != nil {
		return acmm.FixPlan{}, err
	}
	names := make([]string, 0, len(updates))
	for _, u := range updates {
		names = append(names, u.Ecosystem+" "+u.Directory)
	}
	return acmm.FixPlan{
		SignalID: "install-dependabot",
		Summary: fmt.Sprintf("Install %s with %d update block(s) on a %s schedule: %s",
			DefaultPath, len(updates), opts.interval(), strings.Join(names, ", ")),
		Ops: []acmm.FixOp{{
			Kind: acmm.FixCreateFile,
			Path: DefaultPath,
			Body: []byte(body),
		}},
	}, nil
}
