// Package textwrap is a small word-wrap helper shared by the CLI
// inspect formatter and the Bubble Tea TUI detail view. It is
// deliberately ASCII-oriented and stdlib-only — no extra dependency
// just to wrap a paragraph.
package textwrap

import "strings"

// Wrap word-wraps s to width columns, preserving any explicit newlines
// already present. Words longer than width are emitted on their own
// line rather than mid-word split (matches what humans expect when
// inspecting code paths or URLs).
func Wrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	var out strings.Builder
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			out.WriteByte('\n')
		}
		wrapLine(&out, line, width)
	}
	return out.String()
}

// Indent prefixes every line of s with prefix, then word-wraps the
// result so each output line fits in width columns *including* the
// prefix.
func Indent(prefix, s string, width int) string {
	body := Wrap(s, width-len(prefix))
	var out strings.Builder
	for i, line := range strings.Split(body, "\n") {
		if i > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(prefix)
		out.WriteString(line)
	}
	return out.String()
}

func wrapLine(w *strings.Builder, line string, width int) {
	if len(line) <= width {
		w.WriteString(line)
		return
	}
	words := strings.Fields(line)
	if len(words) == 0 {
		// Whitespace-only line; preserve as empty.
		return
	}
	pos := 0
	for i, word := range words {
		switch {
		case i == 0:
			w.WriteString(word)
			pos = len(word)
		case pos+1+len(word) > width:
			w.WriteByte('\n')
			w.WriteString(word)
			pos = len(word)
		default:
			w.WriteByte(' ')
			w.WriteString(word)
			pos += 1 + len(word)
		}
	}
}
