// Package scanner walks a repository and produces a RepoIndex that
// signal detectors consume. The index carries metadata only; content is
// reachable via idx.Read, which enforces the sample-size cap. See
// SPEC.md §5 for the access contract.
package scanner

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/sroberts/plumbline/internal/workflows"
)

// ReadSampleSize is the maximum number of bytes idx.Read returns for any
// single file. Files larger than this are sampled (first ReadSampleSize
// bytes only), bounding memory and IO. See SPEC.md §11.
const ReadSampleSize = 64 * 1024

// defaultIgnoredDirs is the set of directory names skipped during a
// scan unless the caller's config overrides them.
var defaultIgnoredDirs = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	"vendor":       {},
}

// FileEntry is a metadata-only record of a tracked file. Content is
// reachable via RepoIndex.Read.
type FileEntry struct {
	Path string
	Size int64
	Mode fs.FileMode
}

// RepoIndex is the metadata-only view of a repository, handed to every
// signal detector. Its access contract is enforced by lint (signals are
// forbidden from doing direct IO) and documented in SPEC.md §5.
type RepoIndex struct {
	Root      string
	Files     []FileEntry
	ByName    map[string][]string
	Workflows []*workflows.File
	HasGit    bool

	fsys  fs.FS
	cache map[string][]byte

	// nestedRepos holds the prefixes of directories skipped because they
	// are separate repositories. Read refuses paths under them, so a
	// detector cannot reach into another repo by constructing a path the
	// walk deliberately excluded.
	nestedRepos []string
}

// Scan walks the repository at the given filesystem path and returns a
// populated RepoIndex.
func Scan(root string) (*RepoIndex, error) {
	return ScanFS(os.DirFS(root), root)
}

// ScanFS is the testable core. Tests pass an fstest.MapFS; Scan wraps
// it with os.DirFS for the real filesystem.
func ScanFS(fsys fs.FS, root string) (*RepoIndex, error) {
	idx := &RepoIndex{
		Root:   root,
		ByName: make(map[string][]string),
		fsys:   fsys,
		cache:  make(map[string][]byte),
	}

	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}

		// Note .git presence even though its contents aren't indexed.
		if p == ".git" {
			idx.HasGit = true
		}

		if d.IsDir() {
			if _, skip := defaultIgnoredDirs[p]; skip {
				return fs.SkipDir
			}
			// Nested directories of the same name (e.g. project/vendor)
			// are also skipped.
			if _, skip := defaultIgnoredDirs[path.Base(p)]; skip {
				return fs.SkipDir
			}
			// A directory carrying its own .git is a separate repository:
			// a git worktree, a submodule, or a plain nested clone. Its
			// files describe that repo's maturity, not this one's, and
			// indexing them lets a signal cite a second copy of an
			// artifact — which is how a local scan starts disagreeing with
			// the one CI runs on a clean checkout.
			//
			// The name-based list above cannot catch this: a worktree
			// marks itself with a .git *file* holding "gitdir: ...", not
			// a .git directory. Stat covers both shapes.
			if isNestedRepo(fsys, p) {
				idx.nestedRepos = append(idx.nestedRepos, p+"/")
				return fs.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		idx.Files = append(idx.Files, FileEntry{
			Path: p,
			Size: info.Size(),
			Mode: info.Mode(),
		})
		base := path.Base(p)
		idx.ByName[base] = append(idx.ByName[base], p)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Parse GitHub Actions workflow files into a CI-agnostic AST.
	// Other CI systems are deferred (SPEC.md §6 CI-system scope).
	idx.parseWorkflows()
	return idx, nil
}

// isNestedRepo reports whether dir is the root of a repository other than
// the one being scanned, by looking for a .git entry inside it. Both
// shapes count: a .git directory (submodule or nested clone) and a .git
// file (worktree). The scan root is never passed here — WalkDir returns
// early on "." — so the repo under assessment cannot skip itself.
func isNestedRepo(fsys fs.FS, dir string) bool {
	_, err := fs.Stat(fsys, path.Join(dir, ".git"))
	return err == nil
}

func (idx *RepoIndex) parseWorkflows() {
	for _, f := range idx.Files {
		if !strings.HasPrefix(f.Path, ".github/workflows/") {
			continue
		}
		if !(strings.HasSuffix(f.Path, ".yml") || strings.HasSuffix(f.Path, ".yaml")) {
			continue
		}
		data, err := idx.Read(f.Path)
		if err != nil {
			continue
		}
		w, err := workflows.Parse(f.Path, data)
		if err != nil {
			// Unparseable workflow → not the scanner's job to fail; signals
			// can still see the file via idx.Files and idx.Read if they
			// want to do regex fallbacks.
			continue
		}
		idx.Workflows = append(idx.Workflows, w)
	}
}

// Read returns up to ReadSampleSize bytes of the named tracked file.
// Results are cached for the duration of the scan. Read is the only
// sanctioned IO chokepoint for signal detectors.
func (idx *RepoIndex) Read(p string) ([]byte, error) {
	if cached, ok := idx.cache[p]; ok {
		return cached, nil
	}
	// Paths inside a nested repository are not part of this repo, so they
	// read as absent rather than as an error of their own. Detectors
	// already branch on fs.ErrNotExist for "artifact not here", which is
	// exactly what this means.
	for _, prefix := range idx.nestedRepos {
		if strings.HasPrefix(p, prefix) {
			return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrNotExist}
		}
	}
	f, err := idx.fsys.Open(p)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(f, ReadSampleSize)); err != nil {
		return nil, err
	}
	data := buf.Bytes()
	idx.cache[p] = data
	return data, nil
}
