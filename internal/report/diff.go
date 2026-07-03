package report

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// SignalChange records one signal whose status moved between two reports.
// From/To are empty ("") when the signal was absent on that side — i.e.
// a signal added to or removed from the catalog between the two runs.
type SignalChange struct {
	ID   string      `json:"id"`
	From acmm.Status `json:"from,omitempty"`
	To   acmm.Status `json:"to,omitempty"`
}

// Delta is the difference between two reports: how the verdict level
// moved and which signals changed status. It is what `plumbline diff`
// emits (as text or, with --json, verbatim).
type Delta struct {
	OldLevel     acmm.Level             `json:"old_level"`
	NewLevel     acmm.Level             `json:"new_level"`
	OldName      string                 `json:"old_name"`
	NewName      string                 `json:"new_name"`
	LevelChanged bool                   `json:"level_changed"`
	Direction    string                 `json:"direction"` // "up" | "down" | "same"
	Signals      []SignalChange         `json:"signal_changes"`
	LevelScores  map[acmm.Level]float64 `json:"level_scores"` // the new report's scores
}

// Diff computes the Delta from oldR to newR. Signals are matched by ID;
// only status transitions (including add/remove) are reported, since a
// score wiggle that doesn't move status is noise for a maturity trend.
func Diff(oldR, newR acmm.Report) Delta {
	oldStatus := statusByID(oldR)
	newStatus := statusByID(newR)

	ids := map[string]struct{}{}
	for id := range oldStatus {
		ids[id] = struct{}{}
	}
	for id := range newStatus {
		ids[id] = struct{}{}
	}

	var changes []SignalChange
	for id := range ids {
		o, oldOK := oldStatus[id]
		n, newOK := newStatus[id]
		switch {
		case oldOK && newOK && o != n:
			changes = append(changes, SignalChange{ID: id, From: o, To: n})
		case oldOK && !newOK:
			changes = append(changes, SignalChange{ID: id, From: o})
		case !oldOK && newOK:
			changes = append(changes, SignalChange{ID: id, To: n})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].ID < changes[j].ID })

	dir := "same"
	switch {
	case newR.Verdict.Level > oldR.Verdict.Level:
		dir = "up"
	case newR.Verdict.Level < oldR.Verdict.Level:
		dir = "down"
	}

	return Delta{
		OldLevel:     oldR.Verdict.Level,
		NewLevel:     newR.Verdict.Level,
		OldName:      oldR.Verdict.Name,
		NewName:      newR.Verdict.Name,
		LevelChanged: oldR.Verdict.Level != newR.Verdict.Level,
		Direction:    dir,
		Signals:      changes,
		LevelScores:  newR.Verdict.LevelScores,
	}
}

func statusByID(r acmm.Report) map[string]acmm.Status {
	m := make(map[string]acmm.Status, len(r.Signals))
	for _, s := range r.Signals {
		m[s.ID] = s.Status
	}
	return m
}

// RenderDeltaMarkdown formats a Delta as a Markdown block suitable for a
// PR comment. It states the level move (or that it held), lists the
// signals that changed status, and prints the new per-level scores.
func RenderDeltaMarkdown(d Delta) []byte {
	var b bytes.Buffer

	fmt.Fprintln(&b, "## plumbline verdict delta")
	fmt.Fprintln(&b)
	if d.LevelChanged {
		arrow := "↑"
		if d.Direction == "down" {
			arrow = "↓"
		}
		fmt.Fprintf(&b, "Verdict moved **L%d (%s) %s L%d (%s)**.\n",
			d.OldLevel, d.OldName, arrow, d.NewLevel, d.NewName)
	} else {
		fmt.Fprintf(&b, "Verdict unchanged: **L%d (%s)**.\n", d.NewLevel, d.NewName)
	}
	fmt.Fprintln(&b)

	if len(d.Signals) > 0 {
		fmt.Fprintln(&b, "_Signals that moved:_")
		for _, c := range d.Signals {
			switch {
			case c.From == "":
				fmt.Fprintf(&b, "  - `%s`: _(new)_ → %s\n", c.ID, c.To)
			case c.To == "":
				fmt.Fprintf(&b, "  - `%s`: %s → _(removed)_\n", c.ID, c.From)
			default:
				fmt.Fprintf(&b, "  - `%s`: %s → %s\n", c.ID, c.From, c.To)
			}
		}
		fmt.Fprintln(&b)
	} else {
		fmt.Fprintln(&b, "_No signals changed status._")
		fmt.Fprintln(&b)
	}

	fmt.Fprintln(&b, "_Per-level scores:_")
	for _, l := range []acmm.Level{
		acmm.LevelInstructed, acmm.LevelMeasured,
		acmm.LevelAdaptive, acmm.LevelSelfSustaining,
	} {
		fmt.Fprintf(&b, "  - L%d: %d%%\n", l, int(d.LevelScores[l]*100))
	}

	return b.Bytes()
}
