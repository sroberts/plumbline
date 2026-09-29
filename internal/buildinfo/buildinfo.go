// Package buildinfo holds release metadata injected at build time via -ldflags.
package buildinfo

var (
	Version = "dev"
	Commit  = "unknown"
)

// SignalSetVersion is the current frozen signal-set version. Bumping
// this is governed by the SPEC.md §8.2.5 compatibility policy: each
// rename or merge requires a deprecation alias for at least one minor
// version (see internal/signals/aliases.go).
//
// v1 → v2 (this release): l2.claude-md and l2.copilot-instructions
// were merged into l2.agent-instructions. Both old IDs remain valid
// as deprecation aliases that rewrite + warn on use.
//
// v2 → v3: l3.verification-harness became a verdict gate (SPEC.md §7
// "Verification gate"), which tightens the rules: a repo whose per-level
// averages reach L3 but which has no runnable local tests is now held at
// L2. Also added l3.verification-harness, l3.api-contract and
// l3.lint-config. No signal was renamed or removed, so no new aliases.
const SignalSetVersion = "v3"

// Schema is the top-level $id prefix for published JSON Schemas.
const Schema = "plumbline/v1"
