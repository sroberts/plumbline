package scanner

import (
	"strings"
	"testing"
	"testing/fstest"
)

// indexed reports whether any indexed path starts with prefix.
func indexed(idx *RepoIndex, prefix string) []string {
	var hits []string
	for _, f := range idx.Files {
		if strings.HasPrefix(f.Path, prefix) {
			hits = append(hits, f.Path)
		}
	}
	return hits
}

// TestScan_SkipsNestedWorktree is the case that bit repeatedly in this
// repo: Claude Code checks a git worktree out under .claude/worktrees/,
// which is a second full copy of the repository. Walking into it indexes
// every artifact twice, so a signal's evidence can cite the copy instead
// of the real file and a local `plumbline snapshot` stops matching the
// one CI produces from a clean checkout.
//
// A worktree marks itself with a .git *file* (containing "gitdir: ..."),
// not a .git directory, which is why the name-based ignore list never
// caught it.
func TestScan_SkipsNestedWorktree(t *testing.T) {
	files := fstest.MapFS{
		"README.md":                                    {Data: []byte("# real repo\n")},
		"acceptance-rates.json":                        {Data: []byte("{}\n")},
		".git/HEAD":                                    {Data: []byte("ref: refs/heads/main\n")},
		".claude/worktrees/wt-1/.git":                  {Data: []byte("gitdir: /repo/.git/worktrees/wt-1\n")},
		".claude/worktrees/wt-1/README.md":             {Data: []byte("# copy\n")},
		".claude/worktrees/wt-1/acceptance-rates.json": {Data: []byte("{}\n")},
	}
	idx, err := ScanFS(files, "/repo")
	if err != nil {
		t.Fatalf("ScanFS: %v", err)
	}

	if hits := indexed(idx, ".claude/worktrees/"); len(hits) > 0 {
		t.Errorf("indexed %d file(s) inside a nested worktree: %v\n"+
			"those belong to that checkout's assessment, not this repo's", len(hits), hits)
	}
	if got := idx.ByName["acceptance-rates.json"]; len(got) != 1 || got[0] != "acceptance-rates.json" {
		t.Errorf("ByName[acceptance-rates.json] = %v, want exactly the real one; "+
			"a duplicate lets a signal cite the worktree copy", got)
	}
	if len(indexed(idx, "README.md")) != 1 {
		t.Errorf("the real repo's own files must still be indexed")
	}
}

// TestScan_SkipsNestedRepoDir covers the other shape: a submodule or a
// plain nested clone, which carries a .git directory rather than a file.
func TestScan_SkipsNestedRepoDir(t *testing.T) {
	files := fstest.MapFS{
		"README.md":                       {Data: []byte("# real\n")},
		".git/HEAD":                       {Data: []byte("ref: refs/heads/main\n")},
		"third_party/dep/.git/HEAD":       {Data: []byte("ref: refs/heads/main\n")},
		"third_party/dep/CLAUDE.md":       {Data: []byte("# someone else's\n")},
		"third_party/dep/CONTRIBUTING.md": {Data: []byte("# theirs\n")},
	}
	idx, err := ScanFS(files, "/repo")
	if err != nil {
		t.Fatalf("ScanFS: %v", err)
	}
	if hits := indexed(idx, "third_party/dep/"); len(hits) > 0 {
		t.Errorf("indexed a nested repository's files: %v", hits)
	}
	if _, err := idx.Read("third_party/dep/CLAUDE.md"); err == nil {
		t.Errorf("a nested repo's CLAUDE.md must not be readable; it would credit " +
			"this repo for someone else's agent instructions")
	}
}

// TestScan_RootRepoStillScanned guards the obvious over-correction: the
// repo under assessment carries a .git of its own, and skipping on that
// basis would index nothing at all.
func TestScan_RootRepoStillScanned(t *testing.T) {
	files := fstest.MapFS{
		".git/HEAD": {Data: []byte("ref: refs/heads/main\n")},
		"README.md": {Data: []byte("# real\n")},
		"CLAUDE.md": {Data: []byte("# guidance\n")},
		"docs/a.md": {Data: []byte("a\n")},
	}
	idx, err := ScanFS(files, "/repo")
	if err != nil {
		t.Fatalf("ScanFS: %v", err)
	}
	if !idx.HasGit {
		t.Errorf("HasGit = false for a repo with .git")
	}
	for _, want := range []string{"README.md", "CLAUDE.md", "docs/a.md"} {
		if len(indexed(idx, want)) != 1 {
			t.Errorf("%s was not indexed; the root repo must always be scanned", want)
		}
	}
}

// TestScan_OrdinaryDotDirsStillScanned — the fix keys on a nested .git,
// not on a directory name. A plain .claude/ holding skills is ordinary
// repo content and must still be seen.
func TestScan_OrdinaryDotDirsStillScanned(t *testing.T) {
	files := fstest.MapFS{
		".git/HEAD":                         {Data: []byte("ref: refs/heads/main\n")},
		"README.md":                         {Data: []byte("# real\n")},
		".claude/skills/plumbline/SKILL.md": {Data: []byte("# skill\n")},
	}
	idx, err := ScanFS(files, "/repo")
	if err != nil {
		t.Fatalf("ScanFS: %v", err)
	}
	if len(indexed(idx, ".claude/skills/")) != 1 {
		t.Errorf(".claude/ without a nested .git is ordinary content and must be indexed")
	}
}
