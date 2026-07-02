package report

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/sroberts/plumbline/pkg/acmm"
)

func TestDecodeReport_RoundTripsAllFormats(t *testing.T) {
	r := sampleReport()
	r.Signals[0].Evidence = []acmm.Evidence{{Path: "CLAUDE.md", Excerpt: "hi, there: x"}}

	toon, err := TOON(r)
	if err != nil {
		t.Fatal(err)
	}
	yml, err := YAML(r)
	if err != nil {
		t.Fatal(err)
	}
	jsn := mustJSON(t, r)

	for _, tc := range []struct {
		format string
		data   []byte
	}{
		{"toon", toon},
		{"yaml", yml},
		{"json", jsn},
	} {
		t.Run(tc.format, func(t *testing.T) {
			got, err := DecodeReport(tc.data, tc.format)
			if err != nil {
				t.Fatalf("DecodeReport(%s): %v", tc.format, err)
			}
			if !reflect.DeepEqual(got, r) {
				t.Errorf("%s round trip mismatch:\n got: %#v\nwant: %#v", tc.format, got, r)
			}
		})
	}
}

func TestFormatFromPath(t *testing.T) {
	cases := map[string]string{
		".plumbline.toon": "toon",
		"x.json":          "json",
		"x.yaml":          "yaml",
		"x.yml":           "yaml",
		"-":               "toon",
		"noext":           "toon",
	}
	for path, want := range cases {
		if got := FormatFromPath(path); got != want {
			t.Errorf("FormatFromPath(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestDiff_LevelUpAndSignalTransitions(t *testing.T) {
	oldR := acmm.Report{
		Verdict: acmm.Verdict{Level: 2, Name: "Instructed", LevelScores: map[acmm.Level]float64{2: 1, 3: 0.5}},
		Signals: []acmm.SignalResult{
			{ID: "l2.agent-instructions", Status: "found"},
			{ID: "l3.coverage-gate", Status: "missing"},
			{ID: "l3.gone", Status: "partial"},
		},
	}
	newR := acmm.Report{
		Verdict: acmm.Verdict{Level: 3, Name: "Measured", LevelScores: map[acmm.Level]float64{2: 1, 3: 0.8}},
		Signals: []acmm.SignalResult{
			{ID: "l2.agent-instructions", Status: "found"}, // unchanged
			{ID: "l3.coverage-gate", Status: "found"},      // missing -> found
			{ID: "l3.new", Status: "partial"},              // added
		},
	}

	d := Diff(oldR, newR)

	if !d.LevelChanged || d.Direction != "up" || d.NewLevel != 3 {
		t.Errorf("level move wrong: %+v", d)
	}
	want := []SignalChange{
		{ID: "l3.coverage-gate", From: "missing", To: "found"},
		{ID: "l3.gone", From: "partial", To: ""},
		{ID: "l3.new", From: "", To: "partial"},
	}
	if !reflect.DeepEqual(d.Signals, want) {
		t.Errorf("signal changes:\n got: %#v\nwant: %#v", d.Signals, want)
	}
	// The unchanged signal must not appear.
	for _, c := range d.Signals {
		if c.ID == "l2.agent-instructions" {
			t.Errorf("unchanged signal should not be in the delta")
		}
	}
}

func TestDiff_UnchangedRendersHeld(t *testing.T) {
	r := sampleReport()
	d := Diff(r, r)
	if d.LevelChanged || d.Direction != "same" || len(d.Signals) != 0 {
		t.Errorf("expected no-op delta, got %+v", d)
	}
	md := string(RenderDeltaMarkdown(d))
	if !strings.Contains(md, "Verdict unchanged") {
		t.Errorf("expected 'Verdict unchanged' in:\n%s", md)
	}
	if !strings.Contains(md, "No signals changed status") {
		t.Errorf("expected no-signals note in:\n%s", md)
	}
}

func TestRenderDeltaMarkdown_MoveAndTransitions(t *testing.T) {
	d := Delta{
		OldLevel: 2, NewLevel: 3, OldName: "Instructed", NewName: "Measured",
		LevelChanged: true, Direction: "up",
		Signals: []SignalChange{
			{ID: "l3.coverage-gate", From: "missing", To: "found"},
			{ID: "l3.new", To: "partial"},
			{ID: "l3.gone", From: "partial"},
		},
		LevelScores: map[acmm.Level]float64{2: 1, 3: 0.8, 4: 0, 5: 0},
	}
	md := string(RenderDeltaMarkdown(d))
	for _, want := range []string{
		"L2 (Instructed) ↑ L3 (Measured)",
		"`l3.coverage-gate`: missing → found",
		"`l3.new`: _(new)_ → partial",
		"`l3.gone`: partial → _(removed)_",
		"- L3: 80%",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q. Full:\n%s", want, md)
		}
	}
}

func mustJSON(t *testing.T, r acmm.Report) []byte {
	t.Helper()
	out, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return out
}
