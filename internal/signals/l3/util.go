// Package l3 holds Level 3 (Measured) signals — feedback loops that
// produce quantitative signals about AI agent / CI performance. Many
// L3 signals depend on the parsed workflow AST exposed by the scanner.
package l3

import (
	"errors"
	"io/fs"
	"regexp"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/workflows"
)

// fileExists reports whether the path can be read from the index.
func fileExists(idx *scanner.RepoIndex, path string) bool {
	_, err := idx.Read(path)
	return err == nil
}

// readOrEmpty returns idx.Read(path) or empty bytes for ErrNotExist.
func readOrEmpty(idx *scanner.RepoIndex, path string) []byte {
	data, err := idx.Read(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return nil
	}
	return data
}

// anyByNameMatches reports whether any tracked file's basename matches
// re.
func anyByNameMatches(idx *scanner.RepoIndex, re *regexp.Regexp) (string, bool) {
	for base, paths := range idx.ByName {
		if re.MatchString(base) {
			return paths[0], true
		}
	}
	return "", false
}

// anyPathMatches reports whether any tracked file's full path matches re.
func anyPathMatches(idx *scanner.RepoIndex, re *regexp.Regexp) (string, bool) {
	for _, f := range idx.Files {
		if re.MatchString(f.Path) {
			return f.Path, true
		}
	}
	return "", false
}

// findWorkflow returns the first parsed workflow for which pred returns
// true.
func findWorkflow(idx *scanner.RepoIndex, pred func(*workflows.File) bool) *workflows.File {
	for _, w := range idx.Workflows {
		if pred(w) {
			return w
		}
	}
	return nil
}
