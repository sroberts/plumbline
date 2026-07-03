package report

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sroberts/plumbline/pkg/acmm"
)

// The core contract: decoding an encoded report reproduces exactly the
// JSON-generic tree the encoder started from.
func TestTOON_RoundTripReport(t *testing.T) {
	r := sampleReport()
	// Exercise the list form + nested tabular/inline/quoting paths by
	// giving one signal evidence and notes with awkward characters.
	r.Signals[0].Evidence = []acmm.Evidence{
		{Path: "CLAUDE.md", Excerpt: "line one\nline two, with comma \"and quotes\""},
	}
	r.Signals[0].Notes = []string{"plain", "note, with comma", "colon: here"}

	want, err := toGeneric(r)
	if err != nil {
		t.Fatalf("toGeneric: %v", err)
	}
	encoded, err := TOON(r)
	if err != nil {
		t.Fatalf("TOON: %v", err)
	}
	got, err := toonDecode(encoded)
	if err != nil {
		t.Fatalf("toonDecode: %v\n--- encoded ---\n%s", err, encoded)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip mismatch.\n--- encoded ---\n%s\n--- got  ---\n%#v\n--- want ---\n%#v", encoded, got, want)
	}
}

// Round-trip a spread of generic values directly, covering every array
// form and scalar type the encoder can emit.
func TestTOON_RoundTripGenericValues(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]any
	}{
		{"scalars", map[string]any{"s": "hi", "n": 3.5, "i": 2.0, "b": true, "z": nil}},
		{"inline_array", map[string]any{"tags": []any{"a", "b", "c"}}},
		{"empty_array", map[string]any{"gap": []any{}}},
		{"quoted_needs", map[string]any{"vals": []any{"a,b", "c: d", "true", "42", ""}}},
		{"nested_object", map[string]any{"outer": map[string]any{"inner": map[string]any{"x": 1.0}}}},
		{"numeric_keys", map[string]any{"scores": map[string]any{"2": 1.0, "3": 0.75}}},
		{"tabular", map[string]any{"rows": []any{
			map[string]any{"id": "a", "n": 1.0},
			map[string]any{"id": "b", "n": 2.0},
		}}},
		{"list_nonuniform", map[string]any{"items": []any{
			map[string]any{"id": "a"},
			map[string]any{"id": "b", "extra": "y"},
		}}},
		{"multiline_string", map[string]any{"body": "one\ntwo\tthree"}},
		// Strings the encoder emits bare because they are NOT canonical
		// numbers (leading zero, dotted version, exponent) must decode back
		// as strings, not numbers — the repo base-name "001" case.
		{"numeric_looking_strings", map[string]any{"repo": "001", "ver": "1.0.0", "exp": "1e6", "leaddot": "007"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := jsonNormalize(t, c.in)
			encoded := encodeGeneric(c.in)
			got, err := toonDecode(encoded)
			if err != nil {
				t.Fatalf("decode: %v\n%s", err, encoded)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("mismatch.\n--- encoded ---\n%s\n got: %#v\nwant: %#v", encoded, got, want)
			}
		})
	}
}

func TestTOON_DecodeUnterminatedQuoteErrors(t *testing.T) {
	if _, err := toonDecode([]byte("vals[1]: \"unterminated\n")); err == nil {
		t.Errorf("expected error decoding unterminated quote")
	}
}

// jsonNormalize marshals then unmarshals v so all numbers become float64,
// matching what toonDecode produces.
func jsonNormalize(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

// encodeGeneric runs the in-package toonEncoder over a raw generic tree
// (routed through JSON first so numbers/keys match the report path).
func encodeGeneric(v any) []byte {
	b, _ := json.Marshal(v)
	var norm any
	_ = json.Unmarshal(b, &norm)
	var buf bytes.Buffer
	e := &toonEncoder{buf: &buf}
	e.root(norm)
	return buf.Bytes()
}
