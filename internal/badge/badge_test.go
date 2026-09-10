package badge

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/sroberts/plumbline/pkg/acmm"
)

func verdict(l acmm.Level) acmm.Verdict {
	return acmm.Verdict{Level: l, Name: l.Name()}
}

// TestSVG_WellFormedXML is the load-bearing check: a badge that GitHub
// cannot parse renders as a broken-image icon in the README, which is
// worse than no badge at all.
func TestSVG_WellFormedXML(t *testing.T) {
	for _, l := range []acmm.Level{1, 2, 3, 4, 5} {
		out := SVG(verdict(l), Options{})
		dec := xml.NewDecoder(bytes.NewReader(out))
		for {
			_, err := dec.Token()
			if err != nil {
				if err.Error() == "EOF" {
					break
				}
				t.Fatalf("L%d badge is not well-formed XML: %v\n%s", l, err, out)
			}
		}
	}
}

// TestSVG_CarriesVerdictText asserts the level number and name are both
// visible in the rendered message — "L3" alone is jargon, "Measured"
// alone loses the ordering.
func TestSVG_CarriesVerdictText(t *testing.T) {
	out := string(SVG(verdict(acmm.LevelMeasured), Options{}))
	for _, want := range []string{"L3", "Measured", "ACMM"} {
		if !strings.Contains(out, want) {
			t.Errorf("badge SVG missing %q:\n%s", want, out)
		}
	}
}

// TestSVG_ColorTracksLevel: the badge must be readable at a glance, so
// the fill has to move with the level. Adjacent levels sharing a color
// would defeat the point.
func TestSVG_ColorTracksLevel(t *testing.T) {
	seen := map[string]acmm.Level{}
	for _, l := range []acmm.Level{1, 2, 3, 4, 5} {
		c := Color(l)
		if c == "" {
			t.Fatalf("L%d has no color", l)
		}
		if prev, dup := seen[c]; dup {
			t.Errorf("L%d and L%d share color %s", prev, l, c)
		}
		seen[c] = l
		if !strings.HasPrefix(c, "#") {
			t.Errorf("L%d color %q is not a hex literal", l, c)
		}
		if !strings.Contains(string(SVG(verdict(l), Options{})), c) {
			t.Errorf("L%d badge does not use its color %s", l, c)
		}
	}
}

// TestSVG_Deterministic backs the CI drift gate: the same verdict must
// regenerate byte-for-byte or every run would dirty the working tree.
func TestSVG_Deterministic(t *testing.T) {
	a := SVG(verdict(acmm.LevelAdaptive), Options{Label: "maturity"})
	b := SVG(verdict(acmm.LevelAdaptive), Options{Label: "maturity"})
	if !bytes.Equal(a, b) {
		t.Errorf("SVG is not byte-stable across calls")
	}
}

// TestSVG_EscapesText guards against a label breaking out of the markup.
func TestSVG_EscapesText(t *testing.T) {
	out := string(SVG(verdict(acmm.LevelMeasured), Options{Label: `a<b&"c"`}))
	if strings.Contains(out, `a<b&"c"`) {
		t.Errorf("label was interpolated raw into the SVG:\n%s", out)
	}
	if !strings.Contains(out, "&lt;") || !strings.Contains(out, "&amp;") {
		t.Errorf("expected XML-escaped label in output:\n%s", out)
	}
}

// TestSVG_WidthTracksTextLength: a fixed-width badge clips long text.
func TestSVG_WidthTracksTextLength(t *testing.T) {
	short := width(t, SVG(verdict(acmm.LevelMeasured), Options{Label: "a"}))
	long := width(t, SVG(verdict(acmm.LevelMeasured), Options{Label: "a much longer label"}))
	if long <= short {
		t.Errorf("badge width did not grow with the label: %d vs %d", short, long)
	}
}

// TestSVG_UnknownLevel keeps an out-of-range verdict renderable rather
// than emitting a badge with an empty message.
func TestSVG_UnknownLevel(t *testing.T) {
	out := string(SVG(acmm.Verdict{Level: 9, Name: acmm.Level(9).Name()}, Options{}))
	if !strings.Contains(out, "Unknown") {
		t.Errorf("out-of-range level should render as Unknown:\n%s", out)
	}
	if Color(9) == "" {
		t.Errorf("out-of-range level must still have a fallback color")
	}
}

// TestSVG_HasAccessibleName — screen readers and GitHub's image alt
// handling need the verdict as text, not only as glyph paths.
func TestSVG_HasAccessibleName(t *testing.T) {
	out := string(SVG(verdict(acmm.LevelSelfSustaining), Options{}))
	if !strings.Contains(out, `role="img"`) || !strings.Contains(out, "aria-label=") {
		t.Errorf("badge missing role/aria-label:\n%s", out)
	}
	if !strings.Contains(out, "<title>") {
		t.Errorf("badge missing <title>:\n%s", out)
	}
}

// TestSVG_CarriesGeneratedMarker backs the CLI's refusal to overwrite a
// file it did not write: without a marker, `--out README.md` would be
// indistinguishable from regenerating a badge.
func TestSVG_CarriesGeneratedMarker(t *testing.T) {
	out := SVG(verdict(acmm.LevelMeasured), Options{})
	if !IsGenerated(out) {
		t.Errorf("badge is not recognized as plumbline-generated:\n%s", out)
	}
	for _, notABadge := range []string{"# README\n", "<svg><rect/></svg>", ""} {
		if IsGenerated([]byte(notABadge)) {
			t.Errorf("IsGenerated said yes to %q", notABadge)
		}
	}
}

// TestMessage_Format pins the message shown on the badge.
func TestMessage_Format(t *testing.T) {
	if got := Message(verdict(acmm.LevelInstructed)); got != "L2 Instructed" {
		t.Errorf("Message = %q, want %q", got, "L2 Instructed")
	}
}

// width extracts the root <svg width="..."> attribute as a number, so
// callers compare magnitudes rather than strings ("165" < "83").
func width(t *testing.T, svg []byte) int {
	t.Helper()
	var root struct {
		Width int `xml:"width,attr"`
	}
	if err := xml.Unmarshal(svg, &root); err != nil {
		t.Fatalf("unmarshal svg: %v", err)
	}
	if root.Width == 0 {
		t.Fatalf("svg has no width attribute")
	}
	return root.Width
}
