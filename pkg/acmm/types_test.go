package acmm

import "testing"

func TestStatusFromScore(t *testing.T) {
	cases := []struct {
		score float64
		want  Status
	}{
		{ScoreMissing, StatusMissing},
		{ScoreStubbed, StatusPartial},
		{ScoreIncomplete, StatusPartial},
		{ScoreFound, StatusFound},
		{0.5, StatusPartial},
	}
	for _, c := range cases {
		if got := StatusFromScore(c.score); got != c.want {
			t.Errorf("StatusFromScore(%v) = %q, want %q", c.score, got, c.want)
		}
	}
}

func TestConfidenceAtLeast(t *testing.T) {
	cases := []struct {
		got, min Confidence
		want     bool
	}{
		{ConfidenceHigh, ConfidenceHigh, true},
		{ConfidenceHigh, ConfidenceMedium, true},
		{ConfidenceHigh, ConfidenceLow, true},
		{ConfidenceMedium, ConfidenceHigh, false},
		{ConfidenceMedium, ConfidenceMedium, true},
		{ConfidenceMedium, ConfidenceLow, true},
		{ConfidenceLow, ConfidenceHigh, false},
		{ConfidenceLow, ConfidenceMedium, false},
		{ConfidenceLow, ConfidenceLow, true},
	}
	for _, c := range cases {
		if got := c.got.AtLeast(c.min); got != c.want {
			t.Errorf("Confidence(%q).AtLeast(%q) = %v, want %v", c.got, c.min, got, c.want)
		}
	}
}

func TestLevelName(t *testing.T) {
	cases := []struct {
		l    Level
		want string
	}{
		{LevelAssisted, "Assisted"},
		{LevelInstructed, "Instructed"},
		{LevelMeasured, "Measured"},
		{LevelAdaptive, "Adaptive"},
		{LevelSelfSustaining, "Self-Sustaining"},
		{Level(99), "Unknown"},
	}
	for _, c := range cases {
		if got := c.l.Name(); got != c.want {
			t.Errorf("Level(%d).Name() = %q, want %q", c.l, got, c.want)
		}
	}
}
