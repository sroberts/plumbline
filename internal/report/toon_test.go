package report

import (
	"strings"
	"testing"

	"github.com/sroberts/plumbline/pkg/acmm"
)

func TestTOON_TopLevelAndVerdict(t *testing.T) {
	got, err := TOON(sampleReport())
	if err != nil {
		t.Fatalf("TOON: %v", err)
	}
	out := string(got)

	for _, want := range []string{
		"schema: plumbline/v1",
		"tool_version: 1.0.0",
		// scanned_at looks like a timestamp (has colons) → must be quoted.
		`scanned_at: "2026-04-28T15:00:00Z"`,
		"verdict:",
		"  level: 2",
		"  name: Instructed",
		"  level_scores:",
		// Numeric map keys are not bare-key legal, so they are quoted.
		`    "2": 1`,
		`    "3": 0.5`,
		// next_gap is an inline primitive array with a declared length.
		"  next_gap[1]: l3.coverage-gate",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("TOON output missing %q.\nFull output:\n%s", want, out)
		}
	}
}

// sampleReport's two signals share an identical primitive key set, so the
// signals array must collapse to the tabular form with a single header.
func TestTOON_TabularSignals(t *testing.T) {
	out := string(mustTOON(t, sampleReport()))

	if !strings.Contains(out, "signals[2]{confidence,family,id,level,method,score,status}:") {
		t.Errorf("expected tabular signals header, got:\n%s", out)
	}
	// Rows are comma-separated, indented one level (2 spaces) under the
	// top-level header.
	if !strings.Contains(out, "\n  high,instructions,l2.claude-md,2,content-regex,1,found\n") {
		t.Errorf("expected tabular row for l2.claude-md, got:\n%s", out)
	}
	if !strings.Contains(out, "\n  high,coverage,l3.coverage-gate,3,content-regex,0,missing\n") {
		t.Errorf("expected tabular row for l3.coverage-gate, got:\n%s", out)
	}
}

// A non-uniform signals array (evidence present in only some elements)
// falls back to the expanded list form, and evidence/notes exercise the
// nested tabular + inline-array + quoting paths.
func TestTOON_ListFormAndQuoting(t *testing.T) {
	r := sampleReport()
	r.Signals[0].Evidence = []acmm.Evidence{
		{Path: "CLAUDE.md", Excerpt: "line one\nline two, with comma"},
	}
	r.Signals[0].Notes = []string{"plain note", "note, with comma"}

	out := string(mustTOON(t, r))

	// Differing key sets across signals → expanded list, not tabular.
	if strings.Contains(out, "signals[2]{") {
		t.Errorf("expected expanded list form for non-uniform signals, got tabular:\n%s", out)
	}
	if !strings.Contains(out, "signals[2]:") {
		t.Errorf("expected 'signals[2]:' list header, got:\n%s", out)
	}
	// Evidence is a uniform 1-object array → nested tabular.
	if !strings.Contains(out, "evidence[1]{excerpt,path}:") {
		t.Errorf("expected nested tabular evidence header, got:\n%s", out)
	}
	// Excerpt has a newline and comma → quoted with escaped newline.
	if !strings.Contains(out, `"line one\nline two, with comma"`) {
		t.Errorf("expected quoted+escaped excerpt, got:\n%s", out)
	}
	// notes is an inline primitive array; the element containing a comma
	// must be quoted, the plain one must not.
	if !strings.Contains(out, `notes[2]: plain note,"note, with comma"`) {
		t.Errorf("expected inline notes array with per-element quoting, got:\n%s", out)
	}
}

func TestTOON_Deterministic(t *testing.T) {
	a := mustTOON(t, sampleReport())
	b := mustTOON(t, sampleReport())
	if string(a) != string(b) {
		t.Errorf("TOON output is not deterministic:\n--- a ---\n%s\n--- b ---\n%s", a, b)
	}
}

func TestTOON_EmptyArray(t *testing.T) {
	r := sampleReport()
	r.Verdict.NextGap = []string{}
	out := string(mustTOON(t, r))
	if !strings.Contains(out, "next_gap[0]:") {
		t.Errorf("expected 'next_gap[0]:' for empty array, got:\n%s", out)
	}
}

func TestNeedsQuote(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"plain", false},
		{"l2.agent-instructions", false},
		{"a/b/c path", false},
		{"", true},
		{"true", true},
		{"false", true},
		{"null", true},
		{"42", true},
		{"-3.14", true},
		{"1e6", true},
		{"has,comma", true},
		{"has:colon", true},
		{"has\nnewline", true},
		{" leadingspace", true},
		{"-startsdash", true},
	}
	for _, c := range cases {
		if got := needsQuote(c.in); got != c.want {
			t.Errorf("needsQuote(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestFormatNumber(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{2, "2"},
		{0.5, "0.5"},
		{0.67, "0.67"},
		{1000000, "1000000"}, // no exponent, unlike default yaml
	}
	for _, c := range cases {
		if got := formatNumber(c.in); got != c.want {
			t.Errorf("formatNumber(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func mustTOON(t *testing.T, r acmm.Report) []byte {
	t.Helper()
	b, err := TOON(r)
	if err != nil {
		t.Fatalf("TOON: %v", err)
	}
	return b
}
