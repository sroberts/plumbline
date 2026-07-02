package report

import (
	"fmt"
	"strconv"
	"strings"
)

// toonDecode parses a TOON document into the generic map[string]any /
// []any / scalar tree — the exact inverse of the toonEncoder. It is
// deliberately scoped to the surface the encoder emits (2-space indent,
// comma delimiter, the object / inline-array / tabular / list forms); it
// is not a general TOON reader for arbitrary third-party documents. The
// contract that matters is round-trip fidelity: toonDecode(TOON(x)) must
// reproduce x's JSON shape (see toon_decode_test.go).
//
// Numbers decode to float64 and strings/bools/null to their Go
// equivalents, matching a json.Unmarshal into `any`, so callers can
// re-marshal the tree to JSON and unmarshal into a typed struct.
func toonDecode(data []byte) (any, error) {
	d := &toonDecoder{}
	for _, raw := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(raw) == "" {
			continue // encoder never emits blank lines; tolerate trailing \n
		}
		d.lines = append(d.lines, toonLine{indent: leadingSpaces(raw), text: raw[leadingSpaces(raw):]})
	}
	if len(d.lines) == 0 {
		return map[string]any{}, nil
	}

	// Root is a mapping for every plumbline report, but support a root
	// array / scalar too so the decoder is a true inverse of the encoder.
	first := d.lines[0].text
	switch {
	case strings.HasPrefix(first, "["):
		d.pos++
		return d.parseArray(first, 0)
	case isFieldLine(first):
		return d.decodeMapping(0)
	default:
		return parseScalar(first)
	}
}

type toonLine struct {
	indent int
	text   string
}

type toonDecoder struct {
	lines []toonLine
	pos   int
}

// decodeMapping reads consecutive field lines at exactly `indent` into a
// map. Deeper lines are consumed by recursion; a shallower line (or a
// list marker, which never belongs to a mapping) ends the mapping.
func (d *toonDecoder) decodeMapping(indent int) (map[string]any, error) {
	m := map[string]any{}
	for d.pos < len(d.lines) {
		ln := d.lines[d.pos]
		if ln.indent != indent || isListMarker(ln.text) {
			break
		}
		d.pos++
		key, val, err := d.decodeField(ln.text, indent)
		if err != nil {
			return nil, err
		}
		m[key] = val
	}
	return m, nil
}

// decodeField parses a single field from an already-consumed line's text.
// childIndent is the indent of the field's own line; nested content sits
// at childIndent+2. It may consume further lines (object members, array
// rows / items) via recursion.
func (d *toonDecoder) decodeField(text string, childIndent int) (string, any, error) {
	key, rest, err := parseKey(text)
	if err != nil {
		return "", nil, err
	}
	switch {
	case strings.HasPrefix(rest, "["):
		val, err := d.parseArray(rest, childIndent)
		return key, val, err
	case strings.HasPrefix(rest, ":"):
		after := rest[1:]
		if after == "" {
			// Nested object (members at childIndent+2) or empty object.
			obj, err := d.decodeMapping(childIndent + 2)
			return key, obj, err
		}
		val, err := parseScalar(strings.TrimPrefix(after, " "))
		return key, val, err
	default:
		return "", nil, fmt.Errorf("toon: malformed field line %q", text)
	}
}

// parseArray parses an array whose header text starts at '['. childIndent
// is the indent of the header line; rows / items sit at childIndent+2.
func (d *toonDecoder) parseArray(desc string, childIndent int) (any, error) {
	close := strings.IndexByte(desc, ']')
	if !strings.HasPrefix(desc, "[") || close < 0 {
		return nil, fmt.Errorf("toon: malformed array header %q", desc)
	}
	n, err := strconv.Atoi(desc[1:close])
	if err != nil {
		return nil, fmt.Errorf("toon: bad array length in %q: %w", desc, err)
	}
	rest := desc[close+1:]

	// Tabular: [N]{f1,f2}: then N rows at childIndent+2.
	if strings.HasPrefix(rest, "{") {
		end := strings.IndexByte(rest, '}')
		if end < 0 {
			return nil, fmt.Errorf("toon: unterminated tabular header %q", desc)
		}
		fields, err := parseFieldNames(rest[1:end])
		if err != nil {
			return nil, err
		}
		out := make([]any, 0, n)
		for i := 0; i < n; i++ {
			row, ok := d.takeAt(childIndent + 2)
			if !ok {
				return nil, fmt.Errorf("toon: tabular array declared %d rows, got %d", n, i)
			}
			cells, err := splitCSV(row)
			if err != nil {
				return nil, err
			}
			if len(cells) != len(fields) {
				return nil, fmt.Errorf("toon: tabular row has %d cells, want %d", len(cells), len(fields))
			}
			obj := map[string]any{}
			for j, f := range fields {
				v, err := parseScalar(cells[j])
				if err != nil {
					return nil, err
				}
				obj[f] = v
			}
			out = append(out, obj)
		}
		return out, nil
	}

	if !strings.HasPrefix(rest, ":") {
		return nil, fmt.Errorf("toon: malformed array header %q", desc)
	}
	inline := rest[1:]

	// Inline primitive array: "[N]: v1,v2,...".
	if inline != "" {
		parts, err := splitCSV(strings.TrimPrefix(inline, " "))
		if err != nil {
			return nil, err
		}
		out := make([]any, 0, len(parts))
		for _, p := range parts {
			v, err := parseScalar(p)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}

	// Empty array: "[0]:".
	if n == 0 {
		return []any{}, nil
	}

	// Expanded list form: N items at childIndent+2.
	return d.decodeList(childIndent+2, n)
}

// decodeList reads n list items, each beginning with "-" at itemIndent.
func (d *toonDecoder) decodeList(itemIndent, n int) (any, error) {
	out := make([]any, 0, n)
	for i := 0; i < n; i++ {
		if d.pos >= len(d.lines) || d.lines[d.pos].indent != itemIndent || !isListMarker(d.lines[d.pos].text) {
			return nil, fmt.Errorf("toon: list declared %d items, got %d", n, i)
		}
		text := d.lines[d.pos].text
		d.pos++

		head := ""
		if text != "-" {
			head = strings.TrimPrefix(text, "- ")
		}

		switch {
		case head == "":
			out = append(out, map[string]any{})
		case strings.HasPrefix(head, "["):
			// Nested array element (empty key).
			val, err := d.parseArray(head, itemIndent)
			if err != nil {
				return nil, err
			}
			out = append(out, val)
		case isFieldLine(head):
			// Object item: first field shares the hyphen line (its own
			// nested content sits at itemIndent+2), remaining fields follow
			// at itemIndent+2.
			key, val, err := d.decodeField(head, itemIndent+2)
			if err != nil {
				return nil, err
			}
			obj, err := d.decodeMapping(itemIndent + 2)
			if err != nil {
				return nil, err
			}
			obj[key] = val
			out = append(out, obj)
		default:
			v, err := parseScalar(head)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
	}
	return out, nil
}

// takeAt consumes and returns the next line's text if it is at exactly
// `indent`; otherwise it consumes nothing and returns ok=false.
func (d *toonDecoder) takeAt(indent int) (string, bool) {
	if d.pos >= len(d.lines) || d.lines[d.pos].indent != indent {
		return "", false
	}
	t := d.lines[d.pos].text
	d.pos++
	return t, true
}

// ===== line / token helpers =====

func leadingSpaces(s string) int {
	i := 0
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return i
}

func isListMarker(text string) bool {
	return text == "-" || strings.HasPrefix(text, "- ")
}

// isFieldLine reports whether text begins with a "key:" or "key[" form
// (as opposed to a bare scalar). Used to tell an object list item from a
// scalar one.
func isFieldLine(text string) bool {
	_, rest, err := parseKey(text)
	if err != nil {
		return false
	}
	return strings.HasPrefix(rest, ":") || strings.HasPrefix(rest, "[")
}

// parseKey splits a field line into its key and the remainder (which
// starts at the first structural ':' or '['). A quoted key is unescaped;
// a bare key is taken verbatim. For a bare scalar line (no ':' or '['),
// key is the whole text and rest is "".
func parseKey(text string) (string, string, error) {
	if strings.HasPrefix(text, `"`) {
		key, n, err := unquotePrefix(text)
		if err != nil {
			return "", "", err
		}
		return key, text[n:], nil
	}
	idx := -1
	for i := 0; i < len(text); i++ {
		if text[i] == ':' || text[i] == '[' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return text, "", nil
	}
	return text[:idx], text[idx:], nil
}

// parseFieldNames splits a tabular header's "{...}" body into field names,
// unquoting any that were quoted.
func parseFieldNames(s string) ([]string, error) {
	parts, err := splitCSV(s)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(parts))
	for i, p := range parts {
		if strings.HasPrefix(p, `"`) {
			name, _, err := unquotePrefix(p)
			if err != nil {
				return nil, err
			}
			out[i] = name
		} else {
			out[i] = p
		}
	}
	return out, nil
}

// parseScalar inverts encodeScalar: null / bool / number / (quoted)
// string. Numbers become float64 to mirror encoding/json.
func parseScalar(s string) (any, error) {
	switch s {
	case "null":
		return nil, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	if strings.HasPrefix(s, `"`) {
		v, n, err := unquotePrefix(s)
		if err != nil {
			return nil, err
		}
		if n != len(s) {
			return nil, fmt.Errorf("toon: trailing data after quoted string %q", s)
		}
		return v, nil
	}
	// Only treat a bare token as a number when it matches the SAME
	// canonical numeric form the encoder emits (numericRE). This keeps
	// decode a true inverse of encode: the encoder quotes any numeric-
	// looking string, so an unquoted token that matches numericRE must be
	// a real number, while tokens ParseFloat would accept but the encoder
	// never emits bare (e.g. "001" with a leading zero, "1e6") stay
	// strings.
	if numericRE.MatchString(s) {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f, nil
		}
	}
	// Unquoted, non-numeric, non-literal token: a bare string (e.g. an
	// ID like "l2.agent-instructions" or a version like "1.0.0").
	return s, nil
}

// splitCSV splits a delimiter-scoped value list on unquoted commas,
// honoring double-quoted spans (and backslash escapes within them).
func splitCSV(s string) ([]string, error) {
	if s == "" {
		return nil, nil
	}
	var parts []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQuote && c == '\\':
			// Keep the escape pair intact; unquotePrefix decodes it later.
			cur.WriteByte(c)
			if i+1 < len(s) {
				i++
				cur.WriteByte(s[i])
			}
		case c == '"':
			inQuote = !inQuote
			cur.WriteByte(c)
		case c == ',' && !inQuote:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if inQuote {
		return nil, fmt.Errorf("toon: unterminated quoted value in %q", s)
	}
	parts = append(parts, cur.String())
	return parts, nil
}

// unquotePrefix decodes a double-quoted string at the start of s and
// returns the decoded value plus the number of bytes consumed (including
// both quotes). It is the inverse of quoteString.
func unquotePrefix(s string) (string, int, error) {
	if len(s) == 0 || s[0] != '"' {
		return "", 0, fmt.Errorf("toon: expected quoted string, got %q", s)
	}
	var b strings.Builder
	i := 1
	for i < len(s) {
		c := s[i]
		switch c {
		case '"':
			return b.String(), i + 1, nil
		case '\\':
			i++
			if i >= len(s) {
				return "", 0, fmt.Errorf("toon: dangling escape in %q", s)
			}
			switch s[i] {
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'u':
				if i+4 >= len(s) {
					return "", 0, fmt.Errorf("toon: truncated \\u escape in %q", s)
				}
				cp, err := strconv.ParseUint(s[i+1:i+5], 16, 32)
				if err != nil {
					return "", 0, fmt.Errorf("toon: bad \\u escape in %q: %w", s, err)
				}
				b.WriteRune(rune(cp))
				i += 4
			default:
				return "", 0, fmt.Errorf("toon: unknown escape \\%c in %q", s[i], s)
			}
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	return "", 0, fmt.Errorf("toon: unterminated string %q", s)
}
