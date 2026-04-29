//go:build !race

// See internal/scanner/perf_budget_test.go for why these tests are
// gated off the race detector.

package signals_test

import (
	"context"
	"testing"
	"time"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/internal/signals"

	// Side-effect imports populate signals.Default.
	_ "github.com/sroberts/plumbline/internal/signals/l2"
	_ "github.com/sroberts/plumbline/internal/signals/l3"
	_ "github.com/sroberts/plumbline/internal/signals/l4"
	_ "github.com/sroberts/plumbline/internal/signals/l5"
)

// TestPerfBudget_DetectAll_Mid gates the full signal-set against a
// mid-size synthetic repo. Allocations are deterministic so they're
// the load-bearing assertion; the wall-clock check absorbs noise via
// a 5x-of-current ceiling and skips under -short.
const (
	detectMidTimeBudget  = 25 * time.Millisecond
	detectMidAllocBudget = 100 // current: ~47 allocs/op (Apple M2 Max)
)

func TestPerfBudget_DetectAll_Mid(t *testing.T) {
	repo := syntheticRepo(500)
	idx, err := scanner.ScanFS(repo, "/test")
	if err != nil {
		t.Fatal(err)
	}
	all := signals.Default.All()
	if len(all) == 0 {
		t.Fatal("registry empty — side-effect imports missing")
	}
	ctx := context.Background()

	res := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, s := range all {
				_ = s.Detect(ctx, idx)
			}
		}
	})
	if got := res.AllocsPerOp(); got > detectMidAllocBudget {
		t.Errorf("Detect(all)/idx(500) allocs/op = %d; budget %d (regression)",
			got, detectMidAllocBudget)
	}

	if testing.Short() {
		return
	}

	start := time.Now()
	for _, s := range all {
		_ = s.Detect(ctx, idx)
	}
	if elapsed := time.Since(start); elapsed > detectMidTimeBudget {
		t.Errorf("Detect(all) took %v; budget %v (regression)",
			elapsed, detectMidTimeBudget)
	}
}
