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
const SignalSetVersion = "v2"

// Schema is the top-level $id prefix for published JSON Schemas.
const Schema = "plumbline/v1"
