package scanner

import (
	"bytes"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"
)

// helper: collect the paths in a RepoIndex.Files in deterministic order.
func filePaths(idx *RepoIndex) []string {
	paths := make([]string, len(idx.Files))
	for i, f := range idx.Files {
		paths[i] = f.Path
	}
	slices.Sort(paths)
	return paths
}

func TestScan_IndexesAllNonIgnoredFiles(t *testing.T) {
	fsys := fstest.MapFS{
		"README.md":     {Data: []byte("# repo")},
		"CLAUDE.md":     {Data: []byte("# claude\n\nrules")},
		"src/main.go":   {Data: []byte("package main")},
		"docs/intro.md": {Data: []byte("# intro")},
	}

	idx, err := ScanFS(fsys, "/repo")
	if err != nil {
		t.Fatalf("ScanFS: %v", err)
	}
	if idx.Root != "/repo" {
		t.Errorf("Root = %q, want %q", idx.Root, "/repo")
	}

	want := []string{"CLAUDE.md", "README.md", "docs/intro.md", "src/main.go"}
	got := filePaths(idx)
	if !slices.Equal(got, want) {
		t.Errorf("file paths = %v, want %v", got, want)
	}
}

func TestScan_IgnoresDefaultPaths(t *testing.T) {
	fsys := fstest.MapFS{
		"README.md":             {Data: []byte("# repo")},
		".git/config":           {Data: []byte("[core]")},
		".git/HEAD":             {Data: []byte("ref: main")},
		"node_modules/foo/x.js": {Data: []byte("x")},
		"vendor/lib/lib.go":     {Data: []byte("package lib")},
		"src/main.go":           {Data: []byte("package main")},
	}

	idx, err := ScanFS(fsys, ".")
	if err != nil {
		t.Fatalf("ScanFS: %v", err)
	}

	got := filePaths(idx)
	want := []string{"README.md", "src/main.go"}
	if !slices.Equal(got, want) {
		t.Errorf("after default ignores, got %v, want %v", got, want)
	}
}

func TestScan_DetectsGit(t *testing.T) {
	withGit := fstest.MapFS{
		"README.md": {Data: []byte("# r")},
		".git/HEAD": {Data: []byte("ref: main")},
	}
	idx, err := ScanFS(withGit, ".")
	if err != nil {
		t.Fatalf("ScanFS: %v", err)
	}
	if !idx.HasGit {
		t.Errorf("HasGit = false, want true (a .git directory was present)")
	}

	noGit := fstest.MapFS{"README.md": {Data: []byte("# r")}}
	idx, err = ScanFS(noGit, ".")
	if err != nil {
		t.Fatalf("ScanFS: %v", err)
	}
	if idx.HasGit {
		t.Errorf("HasGit = true, want false (no .git directory)")
	}
}

func TestScan_BuildsByNameIndex(t *testing.T) {
	fsys := fstest.MapFS{
		"CLAUDE.md":       {Data: []byte("# top")},
		"docs/CLAUDE.md":  {Data: []byte("# nested")},
		"src/main.go":     {Data: []byte("package main")},
		"src/sub/main.go": {Data: []byte("package sub")},
	}

	idx, err := ScanFS(fsys, ".")
	if err != nil {
		t.Fatalf("ScanFS: %v", err)
	}

	gotClaudes := append([]string(nil), idx.ByName["CLAUDE.md"]...)
	slices.Sort(gotClaudes)
	wantClaudes := []string{"CLAUDE.md", "docs/CLAUDE.md"}
	if !slices.Equal(gotClaudes, wantClaudes) {
		t.Errorf("ByName[CLAUDE.md] = %v, want %v", gotClaudes, wantClaudes)
	}

	gotMains := append([]string(nil), idx.ByName["main.go"]...)
	slices.Sort(gotMains)
	wantMains := []string{"src/main.go", "src/sub/main.go"}
	if !slices.Equal(gotMains, wantMains) {
		t.Errorf("ByName[main.go] = %v, want %v", gotMains, wantMains)
	}

	if got := idx.ByName["does-not-exist"]; got != nil {
		t.Errorf("ByName[does-not-exist] = %v, want nil", got)
	}
}

func TestRead_ReturnsContent(t *testing.T) {
	fsys := fstest.MapFS{
		"CLAUDE.md": {Data: []byte("hello world")},
	}
	idx, _ := ScanFS(fsys, ".")

	got, err := idx.Read("CLAUDE.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("Read = %q, want %q", got, "hello world")
	}
}

func TestRead_CapsAtSampleSize(t *testing.T) {
	big := bytes.Repeat([]byte("x"), 200*1024) // 200 KiB > 64 KiB cap
	fsys := fstest.MapFS{"big.txt": {Data: big}}
	idx, _ := ScanFS(fsys, ".")

	got, err := idx.Read("big.txt")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != ReadSampleSize {
		t.Errorf("Read of 200 KiB file returned %d bytes, want %d (ReadSampleSize)", len(got), ReadSampleSize)
	}
}

func TestRead_NonExistentReturnsError(t *testing.T) {
	fsys := fstest.MapFS{"a.txt": {Data: []byte("a")}}
	idx, _ := ScanFS(fsys, ".")

	if _, err := idx.Read("missing.txt"); err == nil {
		t.Errorf("Read of missing file: expected error, got nil")
	}
}

func TestRead_CachesAcrossCalls(t *testing.T) {
	// A counting fs.FS so we can assert Read is only called once per path.
	count := &countingFS{
		MapFS: fstest.MapFS{"a.txt": {Data: []byte("hi")}},
	}
	idx, _ := ScanFS(count, ".")
	count.opens = 0 // reset; the walk itself doesn't open file content

	if _, err := idx.Read("a.txt"); err != nil {
		t.Fatalf("first Read: %v", err)
	}
	if _, err := idx.Read("a.txt"); err != nil {
		t.Fatalf("second Read: %v", err)
	}
	if count.opens != 1 {
		t.Errorf("Read cache miss: fs.Open was called %d times across 2 reads, want 1", count.opens)
	}
}

// countingFS wraps fstest.MapFS and counts Open calls so we can assert
// that Read caches across calls.
type countingFS struct {
	fstest.MapFS
	opens int
}

func (c *countingFS) Open(name string) (fs.File, error) {
	c.opens++
	return c.MapFS.Open(name)
}
