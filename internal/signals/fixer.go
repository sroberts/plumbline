package signals

import (
	"context"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// Fixer is implemented by signals that know how to scaffold or modify
// their target artifact. Optional — not every signal can auto-fix.
//
// A Fixer is also a Signal. Detect runs as usual; Plan is called only
// when the user explicitly opts in to applying a fix (TUI 'a' key or
// the `plumbline fix <id>` CLI command).
type Fixer interface {
	Signal

	// Inputs lists the user-supplied values Plan needs. Empty for
	// fixes that work from scaffold-only data.
	Inputs() []acmm.FixInput

	// Plan returns the FixPlan to apply, given the user-supplied
	// inputs (keyed by FixInput.Key). Plan MUST NOT touch the file
	// system; it only computes operations. Apply executes them.
	Plan(ctx context.Context, idx *scanner.RepoIndex, inputs map[string]string) (acmm.FixPlan, error)
}
