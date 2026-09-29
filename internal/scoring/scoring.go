// Package scoring rolls per-signal results into a Verdict. The math is
// pinned by SPEC.md §7: per-level average (NA excluded), no level-skipping,
// next_gap names not-yet-Found signals at the level above the verdict.
package scoring

import (
	"slices"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// DefaultPassThreshold is the minimum levelScore required to count a
// level as "achieved." Configurable per-run via Options.PassThreshold.
const DefaultPassThreshold = 0.7

// VerificationGate maps gate signal IDs to the highest level a repo may
// hold while the gate fails. An agent is bounded by how cheaply it can
// verify its work (SPEC.md §7 "Verification gate"): a repo with no
// deterministic local check cannot be Measured, however many dashboards
// it has. Keyed by ID so the scorer needs no import on the signal packages.
var VerificationGate = map[string]acmm.Level{
	"l3.verification-harness": acmm.LevelInstructed,
}

// GateMinScore is the lowest adjusted score a gate signal may carry
// without capping the verdict. 0.67 is "a runnable local test entrypoint
// exists"; below it (0.33 and 0.0) an agent has no one-command way to
// check its own work.
const GateMinScore = acmm.ScoreIncomplete

// Options tune the scoring pass for a single Compute call.
type Options struct {
	PassThreshold float64
	MinConfidence acmm.Confidence
}

// Compute rolls a slice of per-signal results into a Verdict.
func Compute(results []acmm.SignalResult, opts Options) acmm.Verdict {
	threshold := opts.PassThreshold
	if threshold == 0 {
		threshold = DefaultPassThreshold
	}
	minConf := opts.MinConfidence
	if minConf == "" {
		minConf = acmm.ConfidenceLow
	}

	// Group results by level so we can compute per-level averages.
	byLevel := map[acmm.Level][]acmm.SignalResult{}
	for _, r := range results {
		byLevel[r.Level] = append(byLevel[r.Level], r)
	}

	scores := make(map[acmm.Level]float64, 4)
	for _, l := range []acmm.Level{
		acmm.LevelInstructed,
		acmm.LevelMeasured,
		acmm.LevelAdaptive,
		acmm.LevelSelfSustaining,
	} {
		scores[l] = levelScore(byLevel[l], minConf)
	}

	verdictLevel := acmm.LevelAssisted // L1 is the implicit floor.
	for _, l := range []acmm.Level{
		acmm.LevelInstructed,
		acmm.LevelMeasured,
		acmm.LevelAdaptive,
		acmm.LevelSelfSustaining,
	} {
		if scores[l] >= threshold {
			verdictLevel = l
			continue
		}
		// First level that fails the threshold stops the climb — no skipping.
		break
	}

	// The gate applies after the climb so LevelScores stay honest — they
	// report what each level's signals earned, and CappedBy explains why
	// the verdict sits below what they alone would give.
	capLevel, cappedBy := gateCap(results, minConf, verdictLevel)
	verdictLevel = capLevel

	return acmm.Verdict{
		Level:                verdictLevel,
		Name:                 verdictLevel.Name(),
		LevelScores:          scores,
		NextGap:              nextGap(byLevel, verdictLevel),
		MinConfidenceApplied: minConf,
		CappedBy:             cappedBy,
	}
}

// levelScore returns the average score across non-NA signals at a level,
// after applying min-confidence downgrading. An empty (or all-NA) level
// scores 0.0; that means we cannot pass a level we have no evidence for.
func levelScore(results []acmm.SignalResult, minConf acmm.Confidence) float64 {
	sum := 0.0
	n := 0
	for _, r := range results {
		if r.Status == acmm.StatusNA {
			continue
		}
		sum += gateAdjustedScore(r, minConf)
		n++
	}
	if n == 0 {
		return 0.0
	}
	return sum / float64(n)
}

// gateAdjustedScore implements the min-confidence rule from SPEC.md §6:
// signals scoring below 1.0 at confidence below the gate are treated as
// Missing (0.0) for verdict purposes. The maximum score (1.0) is always
// honored regardless of confidence.
func gateAdjustedScore(r acmm.SignalResult, minConf acmm.Confidence) float64 {
	if r.Score >= acmm.ScoreFound {
		return r.Score
	}
	if !r.Confidence.AtLeast(minConf) {
		return 0.0
	}
	return r.Score
}

// nextGap returns the IDs of signals at the level above the verdict
// that are not yet Found, ordered alphabetically by ID for determinism.
// Empty when the verdict is already at the top of the ladder.
func nextGap(byLevel map[acmm.Level][]acmm.SignalResult, verdict acmm.Level) []string {
	if verdict >= acmm.LevelSelfSustaining {
		return []string{}
	}
	target := verdict + 1
	var ids []string
	for _, r := range byLevel[target] {
		if r.Score < acmm.ScoreFound {
			ids = append(ids, r.ID)
		}
	}
	slices.Sort(ids)
	return ids
}

// gateCap returns the level the verdict is held to by failing gate
// signals, and their IDs sorted. A gate signal that is absent from the
// results (disabled in config) or NA never caps: opting out is honored
// the same way it is for level averages. IDs are reported only when the
// gate actually lowered the verdict.
func gateCap(results []acmm.SignalResult, minConf acmm.Confidence, verdict acmm.Level) (acmm.Level, []string) {
	level := verdict
	var ids []string
	for _, r := range results {
		ceiling, isGate := VerificationGate[r.ID]
		if !isGate || r.Status == acmm.StatusNA || ceiling >= verdict {
			continue
		}
		if gateAdjustedScore(r, minConf) >= GateMinScore {
			continue
		}
		ids = append(ids, r.ID)
		level = min(level, ceiling)
	}
	slices.Sort(ids)
	return level, ids
}
