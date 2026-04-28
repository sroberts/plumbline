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

	return acmm.Verdict{
		Level:                verdictLevel,
		Name:                 verdictLevel.Name(),
		LevelScores:          scores,
		NextGap:              nextGap(byLevel, verdictLevel),
		MinConfidenceApplied: minConf,
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
