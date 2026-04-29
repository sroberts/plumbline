// Package plugin runs external signal plugins per SPEC.md §13. A
// plugin is an executable that emits `signal-result` JSON objects on
// stdout when invoked as `<path> --repo <repoRoot> --json`. Plugins
// integrate with the registry through the same Signal interface as
// built-in signals — once loaded, the assess pipeline doesn't know
// or care that a particular signal is plugin-backed.
//
// SPEC.md §13's committed future shape:
//   - Subprocess, not a Go buildmode=plugin (which is brittle and tied
//     to exact toolchain versions)
//   - --repo <path> --json on the command line
//   - signal-result JSON objects on stdout (NDJSON when >1)
//   - SHA-256 attestation: caller pins the expected hash, plugin
//     refuses to load if the file doesn't match
package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// Spec declares one plugin: its executable path and the SHA-256 of
// the binary the user has approved. Empty SHA disables verification
// (useful for local dev only — never set this in CI).
type Spec struct {
	Path   string
	SHA256 string
}

// Plugin wraps a Spec so it satisfies the signals.Signal interface.
// Unlike built-in signals, ID/Level/Family/Title come from the
// plugin's first emitted result rather than being known up-front:
// the signals registry needs an ID to register under, so we probe
// the plugin once at Load time.
type Plugin struct {
	spec    Spec
	id      string
	level   acmm.Level
	family  string
	title   string
	probeOK bool // true once Load successfully read the first result
}

// New constructs a Plugin from a Spec without invoking the binary.
// Useful for tests; production callers should prefer Load.
func New(s Spec) *Plugin { return &Plugin{spec: s} }

// Load probes the plugin against the given repoRoot, verifying its
// SHA-256 (if set) and capturing the first result so the metadata
// fields (ID, Level, Family, Title) are populated.
//
// Probing on Load means a misconfigured plugin trips at startup, not
// halfway through a scan — better failure mode for CI.
func Load(ctx context.Context, s Spec, repoRoot string) (*Plugin, error) {
	if s.Path == "" {
		return nil, errors.New("plugin: empty path")
	}
	if err := verifyChecksum(s); err != nil {
		return nil, err
	}
	p := &Plugin{spec: s}
	results, err := p.invoke(ctx, repoRoot)
	if err != nil {
		return nil, fmt.Errorf("plugin probe %s: %w", s.Path, err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("plugin %s emitted no signal-result on probe", s.Path)
	}
	first := results[0]
	if first.ID == "" {
		return nil, fmt.Errorf("plugin %s: first signal-result has empty id", s.Path)
	}
	p.id = first.ID
	p.level = first.Level
	p.family = first.Family
	p.title = first.Title
	p.probeOK = true
	return p, nil
}

// ID returns the signal ID the plugin advertised at Load time.
func (p *Plugin) ID() string { return p.id }

// Level returns the ACMM level the plugin advertised at Load time.
func (p *Plugin) Level() acmm.Level { return p.level }

// Family returns the signal family the plugin advertised at Load time.
func (p *Plugin) Family() string { return p.family }

// Title returns the human-readable title the plugin advertised at Load time.
func (p *Plugin) Title() string { return p.title }

// Detect runs the plugin and returns the first signal-result whose
// ID matches p.id. Plugins that emit multiple results all under one
// "registered ID" is uncommon; if it happens, only the matching one
// is returned to keep the contract one-signal-per-Detect.
//
// Errors from the subprocess (non-zero exit, malformed JSON) are
// surfaced as a Missing result with diagnostics in Notes — losing a
// plugin shouldn't crash an assess.
func (p *Plugin) Detect(ctx context.Context, idx *scanner.RepoIndex) acmm.Result {
	if !p.probeOK {
		return errResult(p.id, "plugin not loaded; call Load first")
	}
	root := ""
	if idx != nil {
		root = idx.Root
	}
	results, err := p.invoke(ctx, root)
	if err != nil {
		return errResult(p.id, err.Error())
	}
	for _, r := range results {
		if r.ID == p.id {
			return resultFromSignalResult(r)
		}
	}
	return errResult(p.id, fmt.Sprintf("plugin %s did not emit signal-result for %q",
		p.spec.Path, p.id))
}

// invoke executes the plugin and parses NDJSON or single-JSON
// signal-result documents from stdout.
func (p *Plugin) invoke(ctx context.Context, repoRoot string) ([]acmm.SignalResult, error) {
	cmd := exec.CommandContext(ctx, p.spec.Path, "--repo", repoRoot, "--json")
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	return parseSignalResults(stdout.String())
}

// parseSignalResults handles both single-object and NDJSON output.
func parseSignalResults(s string) ([]acmm.SignalResult, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("plugin produced no output")
	}
	// Try a top-level array first (some plugins prefer it).
	if strings.HasPrefix(s, "[") {
		var arr []acmm.SignalResult
		if err := json.Unmarshal([]byte(s), &arr); err != nil {
			return nil, fmt.Errorf("plugin output not a valid signal-result array: %w", err)
		}
		return arr, nil
	}
	// Otherwise NDJSON or one object — line-split and try each.
	var out []acmm.SignalResult
	for i, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var r acmm.SignalResult
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, fmt.Errorf("plugin output line %d not valid signal-result JSON: %w", i+1, err)
		}
		out = append(out, r)
	}
	return out, nil
}

// resultFromSignalResult lifts the embedded Result fields out of a
// SignalResult. Plugins emit the full SignalResult shape (ID + Result),
// but Detect's contract returns only the Result half — the registry
// re-attaches ID at the call site.
func resultFromSignalResult(r acmm.SignalResult) acmm.Result {
	return acmm.Result{
		Status:     r.Status,
		Score:      r.Score,
		Confidence: r.Confidence,
		Method:     r.Method,
		Evidence:   r.Evidence,
		Notes:      r.Notes,
		FixHint:    r.FixHint,
		Diag:       r.Diag,
	}
}

func errResult(id, msg string) acmm.Result {
	return acmm.Result{
		Status:     acmm.StatusMissing,
		Score:      acmm.ScoreMissing,
		Confidence: acmm.ConfidenceLow,
		Method:     acmm.MethodFilenameMatch,
		Notes:      []string{fmt.Sprintf("plugin error (%s): %s", id, msg)},
	}
}

// verifyChecksum reads s.Path and compares its SHA-256 to s.SHA256.
// Returns nil if s.SHA256 is empty (caller opted out of verification).
func verifyChecksum(s Spec) error {
	if s.SHA256 == "" {
		return nil
	}
	f, err := os.Open(s.Path)
	if err != nil {
		return fmt.Errorf("plugin %s: open: %w", s.Path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("plugin %s: hash: %w", s.Path, err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, s.SHA256) {
		return fmt.Errorf("plugin %s: sha256 mismatch (want %s, got %s)",
			s.Path, s.SHA256, got)
	}
	return nil
}

// ParseSpec parses the --plugin flag form `<path>` or `<path>@<sha256>`.
// The @<sha256> half is optional (omit for local dev; require it in CI).
func ParseSpec(s string) (Spec, error) {
	if s == "" {
		return Spec{}, errors.New("plugin spec is empty")
	}
	if i := strings.Index(s, "@"); i >= 0 {
		path := s[:i]
		sha := s[i+1:]
		if path == "" || sha == "" {
			return Spec{}, fmt.Errorf("plugin spec %q: empty path or sha256", s)
		}
		return Spec{Path: path, SHA256: sha}, nil
	}
	return Spec{Path: s}, nil
}
