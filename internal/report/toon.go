package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// TOON encodes an acmm.Report as a TOON (Token-Oriented Object Notation)
// document — the default format for a committable `.plumbline.toon`
// maturity snapshot.
//
// TOON is a compact, indentation-based, LLM-token-efficient
// serialization: uniform arrays of objects collapse to a CSV-like table
// under a single field header, primitive arrays render inline, and every
// array declares its length so a consumer can verify completeness. See
// the spec at https://github.com/toon-format/spec.
//
// The document is produced from the same generic tree as `--report json`
// (the report is marshaled to JSON, then re-decoded), so TOON, JSON, and
// YAML outputs are lossless re-encodings of one another — same keys, same
// omitempty elisions, same numbers. Map keys are emitted in sorted order
// for deterministic, diff-friendly output.
func TOON(r acmm.Report) ([]byte, error) {
	v, err := toGeneric(r)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	e := &toonEncoder{buf: &buf}
	e.root(v)
	return buf.Bytes(), nil
}

// toGeneric round-trips a value through JSON into the generic
// map[string]any / []any / scalar tree that both the TOON and YAML
// encoders consume. Routing through encoding/json is deliberate: it means
// the TOON/YAML key names, omitempty elisions, and value shapes match the
// canonical `--report json` output exactly, so the three formats never
// drift.
func toGeneric(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// toonIndent is the per-level indentation. The spec mandates spaces (tabs
// are forbidden) and a consistent width; 2 is the spec default.
const toonIndent = "  "

type toonEncoder struct {
	buf *bytes.Buffer
}

// root writes the top-level value. A plumbline report is always an
// object, but the array/scalar branches keep the encoder total.
func (e *toonEncoder) root(v any) {
	switch t := v.(type) {
	case map[string]any:
		e.mapping(t, "")
	case []any:
		e.array("", t, "", "")
	default:
		e.buf.WriteString(encodeScalar(v))
		e.buf.WriteByte('\n')
	}
}

// mapping writes every key/value of m at the given indent, keys sorted.
func (e *toonEncoder) mapping(m map[string]any, indent string) {
	for _, k := range sortedKeys(m) {
		e.field(k, m[k], indent, indent)
	}
}

// field writes one key/value pair. linePrefix is the leading text for the
// key's own line — normally the plain indent, but indent+"- " for the
// first field of a list-item object, which shares the hyphen line.
// childIndent is the base indent for any nested content the value emits.
func (e *toonEncoder) field(key string, val any, linePrefix, childIndent string) {
	ek := encodeKey(key)
	switch v := val.(type) {
	case map[string]any:
		// Object: key on its own line, members one level deeper. An empty
		// object emits just "key:" (mapping writes nothing).
		e.buf.WriteString(linePrefix + ek + ":\n")
		e.mapping(v, childIndent+toonIndent)
	case []any:
		e.array(ek, v, linePrefix, childIndent)
	default:
		e.buf.WriteString(linePrefix + ek + ": " + encodeScalar(val) + "\n")
	}
}

// array writes an array value in the most compact legal form: inline for
// all-primitive elements, tabular for uniform objects, else an expanded
// list. header is the key ("" for a root or list-item array).
func (e *toonEncoder) array(header string, arr []any, linePrefix, childIndent string) {
	n := len(arr)
	prefix := linePrefix + header + "[" + strconv.Itoa(n) + "]"

	if n == 0 {
		e.buf.WriteString(prefix + ":\n")
		return
	}

	if allPrimitive(arr) {
		parts := make([]string, n)
		for i, x := range arr {
			parts[i] = encodeScalar(x)
		}
		e.buf.WriteString(prefix + ": " + strings.Join(parts, ",") + "\n")
		return
	}

	if fields, ok := tabularFields(arr); ok {
		cols := make([]string, len(fields))
		for i, f := range fields {
			cols[i] = encodeKey(f)
		}
		e.buf.WriteString(prefix + "{" + strings.Join(cols, ",") + "}:\n")
		rowIndent := childIndent + toonIndent
		for _, x := range arr {
			m := x.(map[string]any)
			cells := make([]string, len(fields))
			for i, f := range fields {
				cells[i] = encodeScalar(m[f])
			}
			e.buf.WriteString(rowIndent + strings.Join(cells, ",") + "\n")
		}
		return
	}

	// Expanded list form for non-uniform or nested arrays.
	e.buf.WriteString(prefix + ":\n")
	itemIndent := childIndent + toonIndent
	for _, x := range arr {
		e.listItem(x, itemIndent)
	}
}

// listItem writes one element of an expanded (non-tabular) array.
func (e *toonEncoder) listItem(val any, indent string) {
	switch v := val.(type) {
	case map[string]any:
		if len(v) == 0 {
			e.buf.WriteString(indent + "-\n")
			return
		}
		keys := sortedKeys(v)
		cont := indent + toonIndent
		// First field shares the hyphen line; the rest align beneath it.
		e.field(keys[0], v[keys[0]], indent+"- ", cont)
		for _, k := range keys[1:] {
			e.field(k, v[k], cont, cont)
		}
	case []any:
		// Nested array element: "- [M]: ..." (the header key is empty).
		e.array("", v, indent+"- ", indent+toonIndent)
	default:
		e.buf.WriteString(indent + "- " + encodeScalar(val) + "\n")
	}
}

// sortedKeys returns m's keys in stable sorted order.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// allPrimitive reports whether every element is a scalar (not an object
// or nested array) — the precondition for the inline array form.
func allPrimitive(arr []any) bool {
	for _, x := range arr {
		switch x.(type) {
		case map[string]any, []any:
			return false
		}
	}
	return true
}

// tabularFields returns the shared field list if arr is a non-empty slice
// of objects that all carry the identical key set and only primitive
// values — the precondition for TOON's tabular array form.
func tabularFields(arr []any) ([]string, bool) {
	first, ok := arr[0].(map[string]any)
	if !ok || len(first) == 0 {
		return nil, false
	}
	fields := sortedKeys(first)
	for _, x := range arr {
		m, ok := x.(map[string]any)
		if !ok || len(m) != len(fields) {
			return nil, false
		}
		for _, f := range fields {
			val, present := m[f]
			if !present {
				return nil, false
			}
			switch val.(type) {
			case map[string]any, []any:
				return nil, false
			}
		}
	}
	return fields, true
}

// bareKeyRE matches keys that may be emitted unquoted per the spec
// (§7.2). Anything else — e.g. the numeric map keys of level_scores — is
// quoted.
var bareKeyRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)

func encodeKey(k string) string {
	if bareKeyRE.MatchString(k) {
		return k
	}
	return quoteString(k)
}

// encodeScalar renders a JSON-decoded scalar as a TOON value.
func encodeScalar(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		return formatNumber(t)
	case json.Number:
		return string(t)
	case string:
		if needsQuote(t) {
			return quoteString(t)
		}
		return t
	default:
		// A JSON-decoded tree only yields the cases above; anything else
		// is quoted defensively rather than emitted raw and unparseable.
		return quoteString(fmt.Sprintf("%v", t))
	}
}

// formatNumber renders a float in TOON's canonical form: no exponent for
// plumbline's numeric range (levels 1–5, scores 0.0–1.0), shortest exact
// decimal, -0 normalized to 0.
func formatNumber(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "null"
	}
	if f == 0 {
		return "0"
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// numericRE matches strings that would be read back as numbers; such
// strings must be quoted so they survive the round-trip as strings.
var numericRE = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

// needsQuote reports whether a string value must be quoted per the spec's
// quoting rules (§7.3).
func needsQuote(s string) bool {
	if s == "" {
		return true
	}
	if s == "true" || s == "false" || s == "null" {
		return true
	}
	if s[0] == '-' {
		return true
	}
	if numericRE.MatchString(s) {
		return true
	}
	if strings.TrimSpace(s) != s {
		return true
	}
	for _, r := range s {
		switch r {
		case ':', '"', '\\', '[', ']', '{', '}', ',', '|', '\t', '\n', '\r':
			return true
		}
		if r < 0x20 {
			return true
		}
	}
	return false
}

// quoteString wraps s in double quotes and applies TOON's escape set.
func quoteString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
