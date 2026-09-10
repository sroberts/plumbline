// Package ciworkflow renders the GitHub Actions workflow that runs
// plumbline against a repository, and the FixPlan that installs it.
//
// The `install-ci` command and the TUI's CI-install picker both consume
// this package, so the workflow a user gets is identical whichever
// interface they drove (SPEC.md §8.2.1 parity).
//
// The rendered workflow is deliberately self-contained — it installs
// plumbline with `go install` rather than referencing the composite
// action at the repo root. A generated file that works the moment it is
// committed beats one that needs a matching action tag to exist first;
// repos that prefer the action can swap the two steps by hand, and
// README documents that form.
package ciworkflow

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// Variant IDs. Each names a different reason to run plumbline in CI;
// a repo that only wants the badge should not be forced into a gate,
// and vice versa.
const (
	// VariantGate blocks the build when the repo drops below a level.
	VariantGate = "gate"
	// VariantBadge keeps the committed README badge honest.
	VariantBadge = "badge"
	// VariantFull does both.
	VariantFull = "full"
)

// DefaultPath is where the workflow is installed with no --out.
const DefaultPath = ".github/workflows/plumbline.yml"

// DefaultBadgePath mirrors `plumbline badge`'s own default output.
const DefaultBadgePath = ".plumbline-badge.svg"

// Variant is one installable workflow shape.
type Variant struct {
	// ID is the stable CLI-visible identifier.
	ID string
	// Name is human-readable, for the picker and --list.
	Name string
	// Desc is a one-line explanation of what the workflow does.
	Desc string
}

// variants is the ordered list, most-complete first — `full` is the
// default and the one most repos want.
var variants = []Variant{
	{
		ID:   VariantFull,
		Name: "Gate + badge",
		Desc: "Fail the build below a level, and keep the README badge current.",
	},
	{
		ID:   VariantGate,
		Name: "Maturity gate",
		Desc: "Fail the build when the repo assesses below a minimum level.",
	},
	{
		ID:   VariantBadge,
		Name: "Badge drift gate",
		Desc: "Regenerate the README badge and fail if the committed copy is stale.",
	},
}

// Variants returns the installable workflow variants in display order.
func Variants() []Variant { return append([]Variant(nil), variants...) }

// VariantByID looks up a variant by its stable ID.
func VariantByID(id string) (Variant, bool) {
	for _, v := range variants {
		if v.ID == id {
			return v, true
		}
	}
	return Variant{}, false
}

// IDs returns the variant IDs in display order.
func IDs() []string {
	out := make([]string, len(variants))
	for i, v := range variants {
		out[i] = v.ID
	}
	return out
}

// Options configure a rendered workflow.
type Options struct {
	// Variant is one of VariantGate / VariantBadge / VariantFull.
	// Empty means VariantFull.
	Variant string

	// FailBelow is the minimum level the gate enforces (2-5). Zero
	// installs the workflow with no gate — useful for a repo that
	// wants the measurement before it wants the enforcement.
	FailBelow int

	// BadgePath is the committed badge location. Empty means
	// DefaultBadgePath.
	BadgePath string

	// Path is where the workflow file is installed. Empty means
	// DefaultPath. Used by NewPlan, ignored by Render.
	Path string
}

func (o Options) variant() string {
	if o.Variant == "" {
		return VariantFull
	}
	return o.Variant
}

func (o Options) badgePath() string {
	if o.BadgePath == "" {
		return DefaultBadgePath
	}
	return o.BadgePath
}

func (o Options) path() string {
	if o.Path == "" {
		return DefaultPath
	}
	return o.Path
}

// validate rejects option combinations that would render a workflow
// which fails on its first run.
func (o Options) validate() error {
	v := o.variant()
	if _, ok := VariantByID(v); !ok {
		return fmt.Errorf("unknown workflow variant %q (available: %s)", v, strings.Join(IDs(), ", "))
	}
	if o.FailBelow != 0 && (o.FailBelow < 2 || o.FailBelow > 5) {
		return fmt.Errorf("--fail-below %d out of range (want 0 for no gate, or 2-5)", o.FailBelow)
	}
	// The badge path is interpolated into the rendered shell script. It
	// is a path inside the repo, so anything that could not be one is a
	// mistake worth catching here rather than in a broken workflow.
	bp := o.badgePath()
	if strings.ContainsAny(bp, "\n\r") {
		return fmt.Errorf("--badge %q: path may not contain newlines", bp)
	}
	if filepath.IsAbs(bp) {
		return fmt.Errorf("--badge %q: must be relative to the repo root", bp)
	}
	return nil
}

// wantsGate reports whether the rendered workflow will contain a gate
// step. A gate needs both a variant that has one and a floor to enforce.
func (o Options) wantsGate() bool {
	v := o.variant()
	return (v == VariantGate || v == VariantFull) && o.FailBelow > 0
}

// wantsBadge reports whether the rendered workflow will contain the
// badge drift-gate step.
func (o Options) wantsBadge() bool {
	v := o.variant()
	return v == VariantBadge || v == VariantFull
}

// shellSingleQuote renders s as a POSIX single-quoted word, safe to
// splice into the generated `run:` script.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Render returns the workflow YAML for the given options. Output is
// deterministic: the workflow is a committed artifact, so identical
// options must produce identical bytes.
func Render(opts Options) (string, error) {
	if err := opts.validate(); err != nil {
		return "", err
	}
	wantGate := opts.wantsGate()
	wantBadge := opts.wantsBadge()

	var b bytes.Buffer
	b.WriteString("# Managed by `plumbline install-ci`. Safe to edit by hand.\n")
	b.WriteString("#\n")
	b.WriteString("# plumbline assesses this repo against the AI Codebase Maturity Model\n")
	b.WriteString("# (ACMM) — which feedback loops exist on disk. Docs: plumbline help ci\n")
	b.WriteString("name: ACMM maturity\n\n")
	b.WriteString("on:\n")
	b.WriteString("  push:\n")
	b.WriteString("    branches: [main]\n")
	b.WriteString("  pull_request:\n\n")
	b.WriteString("permissions:\n")
	b.WriteString("  contents: read\n\n")
	b.WriteString("jobs:\n")
	b.WriteString("  plumbline:\n")
	b.WriteString("    name: ACMM assessment\n")
	b.WriteString("    runs-on: ubuntu-latest\n")
	b.WriteString("    steps:\n")
	b.WriteString("      - uses: actions/checkout@v4\n\n")
	b.WriteString("      - uses: actions/setup-go@v5\n")
	b.WriteString("        with:\n")
	b.WriteString("          go-version: stable\n\n")
	b.WriteString("      - name: install plumbline\n")
	b.WriteString("        run: go install github.com/sroberts/plumbline/cmd/plumbline@latest\n")

	if wantGate {
		b.WriteString("\n      - name: ACMM gate\n")
		b.WriteString("        # Exit 1 when the repo assesses below the floor. Raise the\n")
		b.WriteString("        # floor as loops land; never lower it to make a build pass.\n")
		fmt.Fprintf(&b, "        run: plumbline assess --fail-below %d --quiet .\n", opts.FailBelow)
	}

	if wantBadge {
		// Bind the path to a shell variable once. Splicing it into every
		// command instead would repeat it four times and, in the
		// double-quoted echo lines, let a quote in the path break out of
		// the string.
		b.WriteString("\n      - name: badge drift gate\n")
		b.WriteString("        # The committed badge is a claim about this repo; regenerate\n")
		b.WriteString("        # it and fail if the claim has gone stale. The badge is\n")
		b.WriteString("        # byte-stable for an unchanged verdict, so a clean repo\n")
		b.WriteString("        # produces no diff. To fix a failure: run `plumbline badge`\n")
		b.WriteString("        # and commit the result.\n")
		b.WriteString("        run: |\n")
		fmt.Fprintf(&b, "          badge=%s\n", shellSingleQuote(opts.badgePath()))
		b.WriteString("          plumbline badge --out \"$badge\" .\n")
		b.WriteString("          # `git diff` only compares tracked files, so a badge that\n")
		b.WriteString("          # was generated but never committed would sail through the\n")
		b.WriteString("          # gate below and leave it green forever — a gate that\n")
		b.WriteString("          # verifies nothing. Check it is tracked first.\n")
		b.WriteString("          if ! git ls-files --error-unmatch -- \"$badge\" >/dev/null 2>&1; then\n")
		b.WriteString("            echo \"::error::$badge is not committed — run 'plumbline badge' and commit the result\"\n")
		b.WriteString("            exit 1\n")
		b.WriteString("          fi\n")
		b.WriteString("          if ! git diff --exit-code -- \"$badge\"; then\n")
		b.WriteString("            echo \"::error::$badge is out of date — run 'plumbline badge' and commit the result\"\n")
		b.WriteString("            exit 1\n")
		b.WriteString("          fi\n")
	}

	if !wantGate && !wantBadge {
		// A `gate` variant with --fail-below 0 would otherwise install a
		// workflow that installs plumbline and does nothing with it.
		b.WriteString("\n      - name: ACMM assessment (report only)\n")
		b.WriteString("        # No gate configured. Re-run `plumbline install-ci` with\n")
		b.WriteString("        # --fail-below N once the repo is ready to enforce a floor.\n")
		b.WriteString("        run: plumbline assess --report markdown --out - .\n")
	}

	return b.String(), nil
}

// NewPlan returns the FixPlan that installs the workflow. The op is a
// create-file, so fix.Apply refuses to clobber a workflow the repo
// already has — CI configuration is not something to overwrite silently.
func NewPlan(opts Options) (acmm.FixPlan, error) {
	body, err := Render(opts)
	if err != nil {
		return acmm.FixPlan{}, err
	}
	v, _ := VariantByID(opts.variant())

	// Desc is a sentence in its own right, so join rather than splice it
	// into one — lowercasing it to fit mangled "README" into "readme".
	summary := fmt.Sprintf("Install the %q GitHub Actions workflow at %s. %s",
		v.ID, opts.path(), v.Desc)
	// Only claim a gate floor the rendered workflow actually enforces:
	// the badge variant has no gate step, so --fail-below is inert there
	// and saying otherwise would make the preview lie.
	if opts.wantsGate() {
		summary += fmt.Sprintf(" Gate floor: L%d.", opts.FailBelow)
	} else if opts.FailBelow > 0 {
		summary += fmt.Sprintf(" Note: --fail-below %d is ignored by the %q variant, which installs no gate step.",
			opts.FailBelow, v.ID)
	}

	return acmm.FixPlan{
		SignalID: "install-ci:" + v.ID,
		Summary:  summary,
		Ops: []acmm.FixOp{{
			Kind: acmm.FixCreateFile,
			Path: opts.path(),
			Body: []byte(body),
		}},
	}, nil
}
