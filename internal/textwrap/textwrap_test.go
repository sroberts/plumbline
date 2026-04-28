package textwrap

import (
	"strings"
	"testing"
)

func TestWrap_ShortLineUntouched(t *testing.T) {
	got := Wrap("hello world", 80)
	if got != "hello world" {
		t.Errorf("Wrap = %q, want %q", got, "hello world")
	}
}

func TestWrap_LongLineWraps(t *testing.T) {
	in := "alpha bravo charlie delta echo foxtrot golf"
	got := Wrap(in, 20)
	for _, line := range strings.Split(got, "\n") {
		if len(line) > 20 {
			t.Errorf("line too long (%d > 20): %q", len(line), line)
		}
	}
	// Ensure no words got split.
	if strings.Contains(got, "char-") || strings.Contains(got, "fox-") {
		t.Errorf("Wrap split a word; output:\n%s", got)
	}
}

func TestWrap_PreservesExistingNewlines(t *testing.T) {
	in := "line one\n\nline two"
	got := Wrap(in, 80)
	if got != in {
		t.Errorf("Wrap mangled newlines: got %q, want %q", got, in)
	}
}

func TestWrap_ZeroWidthReturnsInput(t *testing.T) {
	if got := Wrap("abc def", 0); got != "abc def" {
		t.Errorf("zero width should return input unchanged; got %q", got)
	}
}

func TestIndent_PrefixesEveryLine(t *testing.T) {
	got := Indent("> ", "alpha bravo charlie delta", 12)
	for _, line := range strings.Split(got, "\n") {
		if !strings.HasPrefix(line, "> ") {
			t.Errorf("indented line missing prefix: %q", line)
		}
		if len(line) > 12 {
			t.Errorf("indented line over width: %q (len=%d)", line, len(line))
		}
	}
}
