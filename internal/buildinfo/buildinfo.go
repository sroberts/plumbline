// Package buildinfo holds release metadata injected at build time via -ldflags.
package buildinfo

var (
	Version = "dev"
	Commit  = "unknown"
)

// SignalSetVersion is the current frozen signal-set version. Bumping this
// is governed by the SPEC.md §7 compatibility policy.
const SignalSetVersion = "v1"

// Schema is the top-level $id prefix for published JSON Schemas.
const Schema = "plumbline/v1"
