// Package fix executes acmm.FixPlans against a target repository. It
// is the *only* place plumbline writes inside a target repo, and it
// enforces the safety rules listed in SPEC.md §11:
//
//   - Paths must be relative and stay inside repo root.
//   - CreateFile refuses to overwrite an existing file.
//   - AppendFile requires the target to already exist.
//   - DryRun mode produces a result without touching the filesystem.
//   - Unknown FixOpKinds are rejected (no implicit broadening).
package fix

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// Options tunes a single Apply call.
type Options struct {
	// DryRun simulates the plan without touching the filesystem. The
	// returned Result describes what *would* happen.
	DryRun bool
}

// Result reports per-operation outcomes.
type Result struct {
	Operations []OpResult `json:"operations"`
}

// OpResult is one operation's outcome.
type OpResult struct {
	Kind    acmm.FixOpKind `json:"kind"`
	Path    string         `json:"path"`    // absolute, post-validation
	Wrote   bool           `json:"wrote"`   // true only when not DryRun
	Bytes   int            `json:"bytes"`   // body size that was/would be written
	Existed bool           `json:"existed"` // file existed before this op
}

// Apply executes the plan rooted at repoRoot. repoRoot must be an
// absolute path to a directory that already exists.
func Apply(repoRoot string, plan acmm.FixPlan, opts Options) (Result, error) {
	if !filepath.IsAbs(repoRoot) {
		return Result{}, fmt.Errorf("repoRoot must be absolute, got %q", repoRoot)
	}
	rootInfo, err := os.Stat(repoRoot)
	if err != nil {
		return Result{}, fmt.Errorf("repoRoot %q: %w", repoRoot, err)
	}
	if !rootInfo.IsDir() {
		return Result{}, fmt.Errorf("repoRoot %q is not a directory", repoRoot)
	}

	out := Result{Operations: make([]OpResult, 0, len(plan.Ops))}
	for _, op := range plan.Ops {
		res, err := applyOp(repoRoot, op, opts)
		if err != nil {
			return out, err
		}
		out.Operations = append(out.Operations, res)
	}
	return out, nil
}

// applyOp validates one op and (unless DryRun) executes it.
func applyOp(repoRoot string, op acmm.FixOp, opts Options) (OpResult, error) {
	abs, err := safePath(repoRoot, op.Path)
	if err != nil {
		return OpResult{}, err
	}
	res := OpResult{Kind: op.Kind, Path: abs, Bytes: len(op.Body)}
	switch op.Kind {
	case acmm.FixCreateFile:
		_, statErr := os.Stat(abs)
		switch {
		case statErr == nil:
			return OpResult{}, fmt.Errorf("create-file: %q already exists; refusing to overwrite. Remove it manually first or use append", op.Path)
		case !os.IsNotExist(statErr):
			return OpResult{}, fmt.Errorf("create-file: stat %q: %w", op.Path, statErr)
		}
		if opts.DryRun {
			return res, nil
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return OpResult{}, fmt.Errorf("create-file: mkdir parents of %q: %w", op.Path, err)
		}
		if err := os.WriteFile(abs, op.Body, 0o644); err != nil {
			return OpResult{}, fmt.Errorf("create-file: write %q: %w", op.Path, err)
		}
		res.Wrote = true
		return res, nil

	case acmm.FixAppendFile:
		info, statErr := os.Stat(abs)
		if statErr != nil {
			return OpResult{}, fmt.Errorf("append-file: %q does not exist (use create-file or pre-create the file)", op.Path)
		}
		if info.IsDir() {
			return OpResult{}, fmt.Errorf("append-file: %q is a directory", op.Path)
		}
		res.Existed = true
		if opts.DryRun {
			return res, nil
		}
		f, err := os.OpenFile(abs, os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return OpResult{}, fmt.Errorf("append-file: open %q: %w", op.Path, err)
		}
		defer f.Close()
		// Ensure separation from existing content.
		if _, err := f.Write([]byte("\n")); err != nil {
			return OpResult{}, fmt.Errorf("append-file: write separator: %w", err)
		}
		if _, err := f.Write(op.Body); err != nil {
			return OpResult{}, fmt.Errorf("append-file: write body: %w", err)
		}
		res.Wrote = true
		return res, nil

	default:
		return OpResult{}, fmt.Errorf("unknown FixOpKind %q (allowed: create-file, append-file)", op.Kind)
	}
}

// safePath validates that p is a relative path that stays inside root.
// Returns the absolute resolved path on success.
func safePath(root, p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("invalid path: empty")
	}
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("invalid path %q: absolute paths not allowed", p)
	}
	clean := filepath.Clean(p)
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, string(filepath.Separator)+".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid path %q: escape outside repo root", p)
	}
	abs := filepath.Join(root, clean)
	// Defensive double-check — abs should be within root after resolution.
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(abs, rootAbs+string(filepath.Separator)) && abs != rootAbs {
		return "", fmt.Errorf("invalid path %q: resolves outside repo root", p)
	}
	return abs, nil
}
