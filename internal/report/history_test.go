package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sroberts/plumbline/pkg/acmm"
)

func sampleHistorySource() acmm.Report {
	return acmm.Report{
		Schema:           "plumbline/v1",
		ToolVersion:      "test",
		SignalSetVersion: "v1",
		Repo:             "/repo",
		ScannedAt:        "2026-04-28T15:00:00Z",
		Verdict: acmm.Verdict{
			Level:       acmm.LevelInstructed,
			Name:        "Instructed",
			LevelScores: map[acmm.Level]float64{acmm.LevelInstructed: 1.0},
		},
		Signals: []acmm.SignalResult{
			{ID: "a", Status: acmm.StatusFound},
			{ID: "b", Status: acmm.StatusFound},
			{ID: "c", Status: acmm.StatusPartial},
			{ID: "d", Status: acmm.StatusMissing},
			{ID: "e", Status: acmm.StatusNA},
		},
	}
}

func TestSummarizeReport_CountsAndCarriesVerdict(t *testing.T) {
	got := SummarizeReport(sampleHistorySource())
	if got.Level != acmm.LevelInstructed {
		t.Errorf("Level = %v, want Instructed", got.Level)
	}
	if got.LevelName != "Instructed" {
		t.Errorf("LevelName = %q", got.LevelName)
	}
	want := map[acmm.Status]int{
		acmm.StatusFound: 2, acmm.StatusPartial: 1,
		acmm.StatusMissing: 1, acmm.StatusNA: 1,
	}
	for k, v := range want {
		if got.StatusCounts[k] != v {
			t.Errorf("StatusCounts[%s] = %d, want %d", k, got.StatusCounts[k], v)
		}
	}
}

func TestWriteHistoryLine_OneNDJSONLine(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteHistoryLine(&buf, SummarizeReport(sampleHistorySource())); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Count(out, "\n") != 1 {
		t.Errorf("expected exactly one newline; got %q", out)
	}
	var entry HistoryEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &entry); err != nil {
		t.Fatalf("not valid JSON: %v\nline: %q", err, out)
	}
}

func TestAppendHistory_AppendsAcrossInvocations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "maturity.ndjson")

	r := sampleHistorySource()
	if err := AppendHistory(path, SummarizeReport(r)); err != nil {
		t.Fatal(err)
	}
	// Mutate scanned_at so the second entry is distinguishable, then append.
	r.ScannedAt = "2026-04-29T15:00:00Z"
	if err := AppendHistory(path, SummarizeReport(r)); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (content: %q)", len(lines), data)
	}
	for i, line := range lines {
		var e HistoryEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Errorf("line %d not valid JSON: %v\n%s", i, err, line)
		}
	}
}

func TestAppendHistory_CreatesFileIfMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "history.ndjson")

	// AppendHistory should fail cleanly here since we don't auto-mkdir.
	// The error should mention the path, not crash.
	err := AppendHistory(path, SummarizeReport(sampleHistorySource()))
	if err == nil {
		t.Errorf("expected error for missing parent dir, got nil")
	}
	if !strings.Contains(err.Error(), "history") {
		t.Errorf("error should be wrapped with context; got %v", err)
	}
}
