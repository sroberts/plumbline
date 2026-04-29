package scanner

import (
	"fmt"
	"testing"
	"testing/fstest"
)

// syntheticRepo builds a fstest.MapFS that approximates a real repo:
// a handful of agent-instruction files plus n filler source files in
// package directories. n controls the size dimension we benchmark.
func syntheticRepo(n int) fstest.MapFS {
	fs := fstest.MapFS{
		"README.md":                          {Data: []byte("# repo\n")},
		"CLAUDE.md":                          {Data: []byte("# CLAUDE\n\nrules\n")},
		"AGENTS.md":                          {Data: []byte("# AGENTS\n\nrules\n")},
		".github/copilot-instructions.md":    {Data: []byte("# Copilot\n")},
		".github/pull_request_template.md":   {Data: []byte("## Summary\n- [ ] x\n")},
		".github/workflows/ci.yml":           {Data: []byte("name: CI\non: pull_request\njobs: {}\n")},
		".github/workflows/nightly.yml":      {Data: []byte("name: nightly\non:\n  schedule:\n    - cron: '0 0 * * *'\n")},
		".github/ISSUE_TEMPLATE/feedback.md": {Data: []byte("# feedback\n")},
		"CONTRIBUTING.md":                    {Data: []byte("# Contributing\n")},
		"codecov.yml":                        {Data: []byte("coverage:\n  range: 70..90\n")},
	}
	for i := 0; i < n; i++ {
		dir := fmt.Sprintf("internal/pkg%d", i%10)
		fs[fmt.Sprintf("%s/file%d.go", dir, i)] = &fstest.MapFile{
			Data: []byte(fmt.Sprintf("package pkg%d\n\nfunc Fn%d() int { return %d }\n", i%10, i, i)),
		}
	}
	return fs
}

func BenchmarkScan_Small(b *testing.B) { benchScan(b, 50) }
func BenchmarkScan_Mid(b *testing.B)   { benchScan(b, 500) }
func BenchmarkScan_Large(b *testing.B) { benchScan(b, 2000) }

func benchScan(b *testing.B, n int) {
	b.Helper()
	repo := syntheticRepo(n)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		idx, err := ScanFS(repo, "/test")
		if err != nil {
			b.Fatal(err)
		}
		// Touch the result so the compiler can't elide it.
		if len(idx.Files) == 0 {
			b.Fatal("empty index")
		}
	}
}
