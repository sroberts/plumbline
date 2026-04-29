package signals_test

import (
	"context"
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"

	// Side-effect imports register every signal with signals.Default
	// so the detect benchmark exercises the real catalog rather than
	// an empty registry.
	_ "github.com/sroberts/plumbline/internal/signals/l2"
	_ "github.com/sroberts/plumbline/internal/signals/l3"
	_ "github.com/sroberts/plumbline/internal/signals/l4"
	_ "github.com/sroberts/plumbline/internal/signals/l5"
)

// syntheticRepo mirrors scanner.syntheticRepo but lives here so the
// signals package can build a populated index without exposing a test
// helper across packages. Keep the two in rough sync — when one
// changes shape the other usually should too.
func syntheticRepo(n int) fstest.MapFS {
	fs := fstest.MapFS{
		"README.md":                          {Data: []byte("# repo\n")},
		"CLAUDE.md":                          {Data: []byte("# CLAUDE\n\n" + repeat("rule.\n", 25))},
		"AGENTS.md":                          {Data: []byte("# AGENTS\n\nrules\n")},
		".github/copilot-instructions.md":    {Data: []byte("# Copilot\n")},
		".github/pull_request_template.md":   {Data: []byte("## Summary\n- [ ] one\n- [ ] two\n- [ ] three\n")},
		".github/workflows/ci.yml":           {Data: []byte("name: CI\non: pull_request\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: go test --cov-fail-under=70 ./...\n")},
		".github/workflows/nightly.yml":      {Data: []byte("name: nightly\non:\n  schedule:\n    - cron: '0 0 * * *'\njobs: {}\n")},
		".github/ISSUE_TEMPLATE/feedback.md": {Data: []byte("# feedback\n")},
		"CONTRIBUTING.md":                    {Data: []byte("# Contributing\n" + repeat("rule.\n", 25))},
		"codecov.yml":                        {Data: []byte("coverage:\n  range: 70..90\n")},
		".gitmessage":                        {Data: []byte("subject\n\nbody\n")},
	}
	for i := 0; i < n; i++ {
		dir := fmt.Sprintf("internal/pkg%d", i%10)
		fs[fmt.Sprintf("%s/file%d.go", dir, i)] = &fstest.MapFile{
			Data: []byte(fmt.Sprintf("package pkg%d\n\nfunc Fn%d() int { return %d }\n", i%10, i, i)),
		}
	}
	return fs
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

func BenchmarkDetectAll_Small(b *testing.B) { benchDetect(b, 50) }
func BenchmarkDetectAll_Mid(b *testing.B)   { benchDetect(b, 500) }
func BenchmarkDetectAll_Large(b *testing.B) { benchDetect(b, 2000) }

func benchDetect(b *testing.B, n int) {
	b.Helper()
	repo := syntheticRepo(n)
	idx, err := scanner.ScanFS(repo, "/test")
	if err != nil {
		b.Fatal(err)
	}
	all := signals.Default.All()
	if len(all) == 0 {
		b.Fatal("registry empty — side-effect imports missing")
	}
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, s := range all {
			_ = s.Detect(ctx, idx)
		}
	}
}
