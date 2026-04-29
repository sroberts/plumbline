package signals

import (
	"fmt"
	"io"
	"sort"
	"sync"
)

// Alias records a deprecated-to-current signal-ID rename. SPEC.md
// §8.2.5 is the contract: renames go through a deprecation cycle with
// a `compat:` alias for at least one minor version.
//
// Each entry records:
//   - From: the deprecated ID a caller may still pass
//   - To:   the current ID the alias rewrites to
//   - Since: the signal-set version where the rename happened
//   - Reason: a one-line human-readable migration note
//
// New entries are added when a signal is renamed or merged, never
// removed before the next major signal-set bump.
type Alias struct {
	From   string
	To     string
	Since  string
	Reason string
}

// aliases is the package-level alias registry, keyed by the
// deprecated ID for O(1) rewrite lookups. v1 → v2 renames live here.
var aliases = map[string]Alias{
	"l2.claude-md": {
		From:   "l2.claude-md",
		To:     "l2.agent-instructions",
		Since:  "v2",
		Reason: "merged into l2.agent-instructions: any one of CLAUDE.md / AGENTS.md / .github/copilot-instructions.md / .cursorrules / .windsurfrules satisfies the signal",
	},
	"l2.copilot-instructions": {
		From:   "l2.copilot-instructions",
		To:     "l2.agent-instructions",
		Since:  "v2",
		Reason: "merged into l2.agent-instructions: any one of CLAUDE.md / AGENTS.md / .github/copilot-instructions.md / .cursorrules / .windsurfrules satisfies the signal",
	},
}

// warned tracks which deprecated IDs we've already warned about in
// this process — one warning per ID per run, not per occurrence.
// Without this, a CI invocation passing the same alias to both
// --include-signal and --exclude-signal would print twice.
var (
	warnedMu sync.Mutex
	warned   = map[string]bool{}
)

// ResolveID rewrites a deprecated signal ID to its current name.
// The second return is true if a rewrite happened. IDs that aren't
// aliased pass through unchanged with ok=false.
func ResolveID(id string) (string, bool) {
	a, ok := aliases[id]
	if !ok {
		return id, false
	}
	return a.To, true
}

// LookupAlias returns the full Alias entry for a deprecated ID, or
// false if id is not deprecated. Useful when callers want to surface
// the migration note (e.g. in --json verdicts) rather than just the
// rewrite.
func LookupAlias(id string) (Alias, bool) {
	a, ok := aliases[id]
	return a, ok
}

// AllAliases returns every registered alias in deterministic (From) order.
// Used by `plumbline help compatibility` and tests.
func AllAliases() []Alias {
	out := make([]Alias, 0, len(aliases))
	for _, a := range aliases {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].From < out[j].From })
	return out
}

// ResolveIDs rewrites a slice of (possibly deprecated) IDs in place,
// emitting one warning per deprecated ID to w. Pass io.Discard to
// suppress warnings (tests, JSON-only output paths). Returns the
// rewritten slice and the list of aliases that fired (deterministic
// order, suitable for embedding in verdict JSON).
func ResolveIDs(ids []string, w io.Writer) ([]string, []Alias) {
	if len(ids) == 0 {
		return ids, nil
	}
	out := make([]string, len(ids))
	var fired []Alias
	for i, id := range ids {
		if a, ok := aliases[id]; ok {
			out[i] = a.To
			fired = append(fired, a)
			warnOnce(w, a)
			continue
		}
		out[i] = id
	}
	return out, fired
}

func warnOnce(w io.Writer, a Alias) {
	if w == nil || w == io.Discard {
		return
	}
	warnedMu.Lock()
	defer warnedMu.Unlock()
	if warned[a.From] {
		return
	}
	warned[a.From] = true
	fmt.Fprintf(w, "warning: signal %q is deprecated since signal-set %s; using %q instead (%s)\n",
		a.From, a.Since, a.To, a.Reason)
}

// resetWarnedForTest clears the once-per-process warn cache. Tests
// only — exposed via export_test.go.
func resetWarnedForTest() {
	warnedMu.Lock()
	defer warnedMu.Unlock()
	warned = map[string]bool{}
}
