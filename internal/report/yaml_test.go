package report

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestYAML_RoundTrips confirms the YAML encoder emits a valid document
// that decodes back to the same key/value shape as the report — same
// field names as --json, correct verdict, all signals present.
func TestYAML_RoundTrips(t *testing.T) {
	got, err := YAML(sampleReport())
	if err != nil {
		t.Fatalf("YAML: %v", err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(got, &doc); err != nil {
		t.Fatalf("YAML output is not valid YAML: %v\n%s", err, got)
	}

	if doc["schema"] != "plumbline/v1" {
		t.Errorf("schema = %v, want plumbline/v1", doc["schema"])
	}

	verdict, ok := doc["verdict"].(map[string]any)
	if !ok {
		t.Fatalf("verdict is not a mapping: %T", doc["verdict"])
	}
	if verdict["name"] != "Instructed" {
		t.Errorf("verdict.name = %v, want Instructed", verdict["name"])
	}
	if lvl, _ := verdict["level"].(int); lvl != 2 {
		t.Errorf("verdict.level = %v, want 2", verdict["level"])
	}

	signals, ok := doc["signals"].([]any)
	if !ok {
		t.Fatalf("signals is not a sequence: %T", doc["signals"])
	}
	if len(signals) != 2 {
		t.Errorf("len(signals) = %d, want 2", len(signals))
	}
}

// The YAML and JSON outputs must be the same data in two notations:
// decoding either yields structurally identical trees.
func TestYAML_MatchesJSONShape(t *testing.T) {
	r := sampleReport()

	yb, err := YAML(r)
	if err != nil {
		t.Fatalf("YAML: %v", err)
	}
	jb, err := toGeneric(r)
	if err != nil {
		t.Fatalf("toGeneric: %v", err)
	}

	var fromYAML map[string]any
	if err := yaml.Unmarshal(yb, &fromYAML); err != nil {
		t.Fatalf("decode yaml: %v", err)
	}

	jsonMap, ok := jb.(map[string]any)
	if !ok {
		t.Fatalf("json generic is not a map: %T", jb)
	}
	// Compare the top-level key sets — the two encoders must expose the
	// same fields.
	if len(fromYAML) != len(jsonMap) {
		t.Errorf("YAML top-level keys = %d, JSON = %d", len(fromYAML), len(jsonMap))
	}
	for k := range jsonMap {
		if _, present := fromYAML[k]; !present {
			t.Errorf("YAML output missing top-level key %q", k)
		}
	}
}
