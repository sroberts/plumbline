package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// HistoryEntry is one row of the maturity-history NDJSON file. Each
// entry is a compact summary of a verdict — *not* the full report —
// because the file is meant to be appended to repeatedly (per CI run,
// per local invocation) and tracked over time. Storing the full
// report on every assess would balloon the file and obscure the
// signal in noise.
//
// Schema is intentionally narrow and append-only: new fields can be
// added (consumers ignore unknowns) but existing field names will not
// change without a major plumbline version bump, since downstream
// dashboards and scripts pin to them.
type HistoryEntry struct {
	ScannedAt        string                 `json:"scanned_at"`
	Repo             string                 `json:"repo,omitempty"`
	ToolVersion      string                 `json:"tool_version,omitempty"`
	SignalSetVersion string                 `json:"signal_set_version,omitempty"`
	Level            acmm.Level             `json:"level"`
	LevelName        string                 `json:"level_name,omitempty"`
	LevelScores      map[acmm.Level]float64 `json:"level_scores,omitempty"`
	StatusCounts     map[acmm.Status]int    `json:"status_counts"`
}

// SummarizeReport collapses a full Report into a HistoryEntry.
func SummarizeReport(r acmm.Report) HistoryEntry {
	counts := map[acmm.Status]int{
		acmm.StatusFound:   0,
		acmm.StatusPartial: 0,
		acmm.StatusMissing: 0,
		acmm.StatusNA:      0,
	}
	for _, s := range r.Signals {
		counts[s.Status]++
	}
	return HistoryEntry{
		ScannedAt:        r.ScannedAt,
		Repo:             r.Repo,
		ToolVersion:      r.ToolVersion,
		SignalSetVersion: r.SignalSetVersion,
		Level:            r.Verdict.Level,
		LevelName:        r.Verdict.Name,
		LevelScores:      r.Verdict.LevelScores,
		StatusCounts:     counts,
	}
}

// AppendHistory appends one HistoryEntry to the NDJSON file at path,
// creating the file if it does not exist. Each entry is one line so
// the file remains streamable and grep-friendly.
//
// O_APPEND is atomic on POSIX for writes within PIPE_BUF (4 KiB), so
// concurrent CI jobs writing to the same file produce interleaved but
// uncorrupted lines. A history entry is well under 1 KiB.
func AppendHistory(path string, entry HistoryEntry) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer f.Close()
	return writeHistoryLine(f, entry)
}

// WriteHistoryLine encodes entry as one NDJSON line on w. Exposed for
// callers that want to direct history elsewhere (e.g. an in-memory
// buffer in tests, or stdout from a future `plumbline history` subcommand).
func WriteHistoryLine(w io.Writer, entry HistoryEntry) error {
	return writeHistoryLine(w, entry)
}

func writeHistoryLine(w io.Writer, entry HistoryEntry) error {
	b, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal history entry: %w", err)
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}
