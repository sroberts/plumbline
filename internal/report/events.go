package report

import (
	"encoding/json"
	"io"
	"time"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// EventEmitter writes NDJSON event lines to the configured writer per
// SPEC.md §8.2.4. Concurrency: not safe; call from a single goroutine
// (the assess pipeline runs sequentially in MVP).
type EventEmitter struct {
	w       io.Writer
	enabled bool
	clock   func() time.Time
}

// NewEventEmitter returns an emitter writing to w. Pass enabled=false
// to make every method a no-op (preserves call-site simplicity in the
// pipeline).
func NewEventEmitter(w io.Writer, enabled bool) *EventEmitter {
	return &EventEmitter{w: w, enabled: enabled, clock: time.Now}
}

// SetClock overrides the timestamp source. Used by tests for byte-stable
// golden output.
func (e *EventEmitter) SetClock(now func() time.Time) { e.clock = now }

func (e *EventEmitter) write(payload map[string]interface{}) {
	if !e.enabled {
		return
	}
	payload["ts"] = e.clock().UTC().Format(time.RFC3339Nano)
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_, _ = e.w.Write(append(data, '\n'))
}

func (e *EventEmitter) ScanStart(repo string, signalCount int) {
	e.write(map[string]interface{}{
		"event":        "scan.start",
		"repo":         repo,
		"signal_count": signalCount,
	})
}

func (e *EventEmitter) SignalStart(id string) {
	e.write(map[string]interface{}{
		"event": "signal.start",
		"id":    id,
	})
}

func (e *EventEmitter) SignalComplete(r acmm.SignalResult, durationMs int64) {
	e.write(map[string]interface{}{
		"event":       "signal.complete",
		"id":          r.ID,
		"status":      r.Status,
		"score":       r.Score,
		"duration_ms": durationMs,
	})
}

func (e *EventEmitter) ScanComplete(level acmm.Level, durationMs int64) {
	e.write(map[string]interface{}{
		"event":       "scan.complete",
		"level":       level,
		"duration_ms": durationMs,
	})
}
