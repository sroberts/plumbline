package scoring

import (
	"slices"
	"testing"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// helper: build a SignalResult quickly.
func sr(id string, level acmm.Level, score float64, conf acmm.Confidence) acmm.SignalResult {
	return acmm.SignalResult{
		ID:         id,
		Level:      level,
		Family:     "test",
		Status:     acmm.StatusFromScore(score),
		Score:      score,
		Confidence: conf,
		Method:     acmm.MethodContentRegex,
	}
}

func TestCompute_NoSignalsIsLevelOne(t *testing.T) {
	v := Compute(nil, Options{})
	if v.Level != acmm.LevelAssisted {
		t.Errorf("Level = %d, want 1 (Assisted) for empty input", v.Level)
	}
	if v.Name != "Assisted" {
		t.Errorf("Name = %q, want Assisted", v.Name)
	}
}

func TestCompute_SingleL2FoundReachesLevelTwo(t *testing.T) {
	results := []acmm.SignalResult{
		sr("l2.x", acmm.LevelInstructed, acmm.ScoreFound, acmm.ConfidenceHigh),
	}
	v := Compute(results, Options{})
	if v.Level != acmm.LevelInstructed {
		t.Errorf("Level = %d, want 2", v.Level)
	}
	if got := v.LevelScores[acmm.LevelInstructed]; got != 1.0 {
		t.Errorf("LevelScores[L2] = %v, want 1.0", got)
	}
}

func TestCompute_SingleL2MissingStaysLevelOne(t *testing.T) {
	results := []acmm.SignalResult{
		sr("l2.x", acmm.LevelInstructed, acmm.ScoreMissing, acmm.ConfidenceHigh),
	}
	v := Compute(results, Options{})
	if v.Level != acmm.LevelAssisted {
		t.Errorf("Level = %d, want 1 (L2 below threshold)", v.Level)
	}
}

func TestCompute_PassThresholdIsAverage(t *testing.T) {
	// Two L2 signals; one Found, one Stubbed → avg = 0.665 → below 0.7 default.
	results := []acmm.SignalResult{
		sr("l2.a", acmm.LevelInstructed, acmm.ScoreFound, acmm.ConfidenceHigh),
		sr("l2.b", acmm.LevelInstructed, acmm.ScoreStubbed, acmm.ConfidenceHigh),
	}
	v := Compute(results, Options{})
	if v.Level != acmm.LevelAssisted {
		t.Errorf("avg=0.665 should NOT pass default 0.7 threshold; Level = %d", v.Level)
	}

	// Found + Incomplete → avg = 0.835 → passes 0.7.
	results[1] = sr("l2.b", acmm.LevelInstructed, acmm.ScoreIncomplete, acmm.ConfidenceHigh)
	v = Compute(results, Options{})
	if v.Level != acmm.LevelInstructed {
		t.Errorf("avg=0.835 should pass 0.7 threshold; Level = %d", v.Level)
	}
}

func TestCompute_NoSkipRule(t *testing.T) {
	// L2 missing, L3 perfect → verdict is L1 because we can't skip L2.
	results := []acmm.SignalResult{
		sr("l2.x", acmm.LevelInstructed, acmm.ScoreMissing, acmm.ConfidenceHigh),
		sr("l3.y", acmm.LevelMeasured, acmm.ScoreFound, acmm.ConfidenceHigh),
	}
	v := Compute(results, Options{})
	if v.Level != acmm.LevelAssisted {
		t.Errorf("Level = %d, want 1 (no-skip rule: L2 must pass before L3 counts)", v.Level)
	}
	// LevelScores still record what was observed at each level.
	if v.LevelScores[acmm.LevelMeasured] != 1.0 {
		t.Errorf("LevelScores[L3] = %v, want 1.0 (raw score is preserved even when level is gated)", v.LevelScores[acmm.LevelMeasured])
	}
}

func TestCompute_HighestPassingLevelWins(t *testing.T) {
	// L2 perfect, L3 perfect, L4 missing.
	results := []acmm.SignalResult{
		sr("l2.x", acmm.LevelInstructed, acmm.ScoreFound, acmm.ConfidenceHigh),
		sr("l3.y", acmm.LevelMeasured, acmm.ScoreFound, acmm.ConfidenceHigh),
		sr("l4.z", acmm.LevelAdaptive, acmm.ScoreMissing, acmm.ConfidenceHigh),
	}
	v := Compute(results, Options{})
	if v.Level != acmm.LevelMeasured {
		t.Errorf("Level = %d, want 3", v.Level)
	}
}

func TestCompute_NextGapIsLevelPlusOne(t *testing.T) {
	// L2 perfect, L3 partial → at L2, gap = the missing L3 signals.
	results := []acmm.SignalResult{
		sr("l2.x", acmm.LevelInstructed, acmm.ScoreFound, acmm.ConfidenceHigh),
		sr("l3.coverage", acmm.LevelMeasured, acmm.ScoreMissing, acmm.ConfidenceHigh),
		sr("l3.nightly", acmm.LevelMeasured, acmm.ScoreIncomplete, acmm.ConfidenceHigh),
		sr("l3.flaky", acmm.LevelMeasured, acmm.ScoreFound, acmm.ConfidenceHigh),
	}
	v := Compute(results, Options{})

	// next_gap names signals at L+1 (=L3) that are not Found, ordered by ID.
	want := []string{"l3.coverage", "l3.nightly"}
	if !slices.Equal(v.NextGap, want) {
		t.Errorf("NextGap = %v, want %v", v.NextGap, want)
	}
}

func TestCompute_NextGapEmptyAtLevelFive(t *testing.T) {
	// All L2..L5 perfect → verdict is L5 → no next gap.
	results := []acmm.SignalResult{
		sr("l2.x", acmm.LevelInstructed, acmm.ScoreFound, acmm.ConfidenceHigh),
		sr("l3.y", acmm.LevelMeasured, acmm.ScoreFound, acmm.ConfidenceHigh),
		sr("l4.z", acmm.LevelAdaptive, acmm.ScoreFound, acmm.ConfidenceHigh),
		sr("l5.w", acmm.LevelSelfSustaining, acmm.ScoreFound, acmm.ConfidenceHigh),
	}
	v := Compute(results, Options{})
	if v.Level != acmm.LevelSelfSustaining {
		t.Fatalf("Level = %d, want 5", v.Level)
	}
	if len(v.NextGap) != 0 {
		t.Errorf("NextGap = %v, want empty at L5", v.NextGap)
	}
}

func TestCompute_NAExcludedFromAverage(t *testing.T) {
	results := []acmm.SignalResult{
		sr("l2.applies", acmm.LevelInstructed, acmm.ScoreFound, acmm.ConfidenceHigh),
		// NA signal — must not drag the average down.
		{
			ID: "l2.na", Level: acmm.LevelInstructed, Family: "test",
			Status: acmm.StatusNA, Score: 0.0, Confidence: acmm.ConfidenceHigh,
		},
	}
	v := Compute(results, Options{})
	if got := v.LevelScores[acmm.LevelInstructed]; got != 1.0 {
		t.Errorf("LevelScores[L2] with one NA = %v, want 1.0", got)
	}
	if v.Level != acmm.LevelInstructed {
		t.Errorf("Level = %d, want 2 (NA must not block the verdict)", v.Level)
	}
}

func TestCompute_MinConfidenceDowngradesPartialScores(t *testing.T) {
	// Score=0.67 at Medium confidence: counts at default min-confidence=Low,
	// is downgraded to 0 at min-confidence=High.
	results := []acmm.SignalResult{
		sr("l2.x", acmm.LevelInstructed, acmm.ScoreIncomplete, acmm.ConfidenceMedium),
	}
	low := Compute(results, Options{MinConfidence: acmm.ConfidenceLow})
	if low.LevelScores[acmm.LevelInstructed] != acmm.ScoreIncomplete {
		t.Errorf("at min=low, level score = %v, want %v", low.LevelScores[acmm.LevelInstructed], acmm.ScoreIncomplete)
	}

	high := Compute(results, Options{MinConfidence: acmm.ConfidenceHigh})
	if high.LevelScores[acmm.LevelInstructed] != 0.0 {
		t.Errorf("at min=high, partial+medium should be downgraded to 0; got %v", high.LevelScores[acmm.LevelInstructed])
	}

	// But Score=1.0 (fully Found) at Medium confidence is NOT downgraded —
	// the maximum score is trusted regardless of confidence.
	results = []acmm.SignalResult{
		sr("l2.y", acmm.LevelInstructed, acmm.ScoreFound, acmm.ConfidenceMedium),
	}
	v := Compute(results, Options{MinConfidence: acmm.ConfidenceHigh})
	if v.LevelScores[acmm.LevelInstructed] != 1.0 {
		t.Errorf("Found at Medium should keep 1.0 even at min=high; got %v", v.LevelScores[acmm.LevelInstructed])
	}
}

func TestCompute_VerdictRecordsMinConfidenceApplied(t *testing.T) {
	v := Compute(nil, Options{MinConfidence: acmm.ConfidenceHigh})
	if v.MinConfidenceApplied != acmm.ConfidenceHigh {
		t.Errorf("MinConfidenceApplied = %q, want high", v.MinConfidenceApplied)
	}

	// Default (zero value) → low.
	v = Compute(nil, Options{})
	if v.MinConfidenceApplied != acmm.ConfidenceLow {
		t.Errorf("default MinConfidenceApplied = %q, want low", v.MinConfidenceApplied)
	}
}

func TestCompute_CustomThreshold(t *testing.T) {
	// 0.67 at default 0.7 → fails. At 0.6 → passes.
	results := []acmm.SignalResult{
		sr("l2.x", acmm.LevelInstructed, acmm.ScoreIncomplete, acmm.ConfidenceHigh),
	}
	if v := Compute(results, Options{}); v.Level != acmm.LevelAssisted {
		t.Errorf("with default threshold, level = %d, want 1", v.Level)
	}
	if v := Compute(results, Options{PassThreshold: 0.6}); v.Level != acmm.LevelInstructed {
		t.Errorf("with threshold=0.6, level = %d, want 2", v.Level)
	}
}
