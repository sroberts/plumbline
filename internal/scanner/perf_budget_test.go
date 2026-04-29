//go:build !race

// Perf-budget tests are gated off the race detector. -race adds ~10x
// allocation overhead from synchronization bookkeeping, which would
// either force the budget so loose it stops catching real regressions
// or make the gate flake on race builds. CI runs this file in a
// dedicated non-race job; the main `go test -race` step still runs
// every other test in this package.

package scanner

import (
	"testing"
	"time"
)

// TestPerfBudget_Scan_Mid is a regression gate, not a benchmark. It
// runs the same workload as BenchmarkScan_Mid and fails when either:
//
//   - The mid-size scan takes longer than scanMidTimeBudget (catches
//     accidental quadratic blowups; set very generously to absorb
//     CI-runner variance).
//   - Allocations per scan exceed scanMidAllocBudget (deterministic,
//     so this is the load-bearing gate — any new allocation in the
//     hot path will trip it).
//
// `go test -short` skips the wall-clock check (still gates allocs).
const (
	scanMidTimeBudget  = 50 * time.Millisecond
	scanMidAllocBudget = 1500 // current: ~1405 allocs/op (Apple M2 Max)
)

func TestPerfBudget_Scan_Mid(t *testing.T) {
	repo := syntheticRepo(500)

	// Allocations gate — deterministic, run via testing.Benchmark.
	res := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := ScanFS(repo, "/test"); err != nil {
				b.Fatal(err)
			}
		}
	})
	if got := res.AllocsPerOp(); got > scanMidAllocBudget {
		t.Errorf("Scan(500 files) allocs/op = %d; budget %d (regression)",
			got, scanMidAllocBudget)
	}

	if testing.Short() {
		return
	}

	// Wall-clock gate — generous, single-shot.
	start := time.Now()
	if _, err := ScanFS(repo, "/test"); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > scanMidTimeBudget {
		t.Errorf("Scan(500 files) took %v; budget %v (regression)",
			elapsed, scanMidTimeBudget)
	}
}
