package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sroberts/plumbline/pkg/acmm"
)

func sampleReport() acmm.Report {
	return acmm.Report{
		Schema:           "plumbline/v1",
		ToolVersion:      "1.0.0",
		SignalSetVersion: "v1",
		CISystem:         "github-actions",
		Repo:             "/abs/path",
		ScannedAt:        "2026-04-28T15:00:00Z",
		Verdict: acmm.Verdict{
			Level:                acmm.LevelInstructed,
			Name:                 "Instructed",
			LevelScores:          map[acmm.Level]float64{2: 1.0, 3: 0.5, 4: 0, 5: 0},
			NextGap:              []string{"l3.coverage-gate"},
			MinConfidenceApplied: acmm.ConfidenceLow,
		},
		Signals: []acmm.SignalResult{
			{ID: "l2.claude-md", Level: 2, Family: "instructions", Status: "found", Score: 1.0, Confidence: "high", Method: "content-regex"},
			{ID: "l3.coverage-gate", Level: 3, Family: "coverage", Status: "missing", Score: 0.0, Confidence: "high", Method: "content-regex"},
		},
	}
}

func TestMarkdown_ContainsExpectedSections(t *testing.T) {
	got := string(Markdown(sampleReport()))
	for _, want := range []string{
		"# Plumbline Maturity Report",
		"**Level 2 — Instructed**",
		"Next-level gap (to reach L3)",
		"`l3.coverage-gate`",
		"L2 — Instructed",
		"L3 — Measured",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("markdown missing %q. Output:\n%s", want, got)
		}
	}
}

func TestEventEmitter_NDJSON(t *testing.T) {
	var buf bytes.Buffer
	e := NewEventEmitter(&buf, true)
	fixed := time.Date(2026, 4, 28, 15, 0, 0, 0, time.UTC)
	e.SetClock(func() time.Time { return fixed })

	e.ScanStart("/abs/path", 22)
	e.SignalStart("l2.claude-md")
	e.SignalComplete(acmm.SignalResult{ID: "l2.claude-md", Status: "found", Score: 1.0}, 5)
	e.ScanComplete(acmm.LevelInstructed, 100)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4: %q", len(lines), buf.String())
	}

	for _, line := range lines {
		var ev map[string]interface{}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Errorf("event line is not JSON: %q (err: %v)", line, err)
		}
		if _, ok := ev["event"]; !ok {
			t.Errorf("event line missing 'event' field: %q", line)
		}
		if _, ok := ev["ts"]; !ok {
			t.Errorf("event line missing 'ts' field: %q", line)
		}
	}
}

func TestEventEmitter_DisabledIsNoop(t *testing.T) {
	var buf bytes.Buffer
	e := NewEventEmitter(&buf, false)
	e.ScanStart("/x", 5)
	e.ScanComplete(acmm.LevelInstructed, 100)
	if buf.Len() != 0 {
		t.Errorf("disabled emitter wrote %d bytes, want 0", buf.Len())
	}
}
