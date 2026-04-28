package l2

import "bytes"

// containsHeading reports whether data contains at least one ATX-style
// markdown heading (# / ## / ###...) at the start of any line.
func containsHeading(data []byte) bool {
	for _, line := range bytes.Split(data, []byte("\n")) {
		trimmed := bytes.TrimLeft(line, " \t")
		if len(trimmed) > 0 && trimmed[0] == '#' {
			return true
		}
	}
	return false
}

// countNonBlankLines returns the number of lines containing at least
// one non-whitespace character.
func countNonBlankLines(data []byte) int {
	n := 0
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			n++
		}
	}
	return n
}

// countMarkdownCheckboxes counts occurrences of "- [ ]" (with optional
// leading whitespace) at the start of any line. Used by signals that
// score PR templates and similar checklists.
func countMarkdownCheckboxes(data []byte) int {
	n := 0
	for _, line := range bytes.Split(data, []byte("\n")) {
		trimmed := bytes.TrimLeft(line, " \t")
		if bytes.HasPrefix(trimmed, []byte("- [ ]")) || bytes.HasPrefix(trimmed, []byte("- [x]")) || bytes.HasPrefix(trimmed, []byte("- [X]")) {
			n++
		}
	}
	return n
}

// excerpt returns the first n bytes of data, with a trailing ellipsis
// if truncated. Used in Evidence so the verdict has a citation.
func excerpt(data []byte, n int) string {
	if len(data) <= n {
		return string(data)
	}
	return string(data[:n]) + "…"
}
