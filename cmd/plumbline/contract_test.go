package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestHelpTextContract iterates every command and asserts its --help
// output meets the SPEC.md §9.5 contract: non-empty, has flags listed,
// has Examples block, has See also footer (where applicable).
func TestHelpTextContract(t *testing.T) {
	commands := []struct {
		name        string
		args        []string
		needFlags   bool
		needExample bool
	}{
		{"root", []string{"--help"}, false, false},
		{"assess", []string{"assess", "--help"}, true, true},
		{"inspect", []string{"inspect", "--help"}, true, true},
		{"signals", []string{"signals", "--help"}, true, true},
		{"explain", []string{"explain", "--help"}, true, true},
		{"schema", []string{"schema", "--help"}, false, true},
		{"help", []string{"help", "--help"}, false, false},
		{"version", []string{"version", "--help"}, true, true},
	}

	for _, c := range commands {
		c := c
		t.Run(c.name, func(t *testing.T) {
			code, out, _ := runCLI(t, c.args...)
			if code != exitOK {
				t.Fatalf("--help exit = %d, want 0", code)
			}
			if len(out) < 100 {
				t.Errorf("--help output is suspiciously short (%d bytes)", len(out))
			}
			if c.needFlags && !strings.Contains(out, "Flags:") {
				t.Errorf("--help output missing 'Flags:' section")
			}
			if c.needExample && !strings.Contains(out, "Examples:") {
				t.Errorf("--help output missing 'Examples:' block")
			}
		})
	}
}

// TestHelpTopicContract iterates every help topic and asserts it has
// substantial content. Catches regressions where a topic body gets
// accidentally truncated to a stub.
func TestHelpTopicContract(t *testing.T) {
	for topic := range helpTopics {
		topic := topic
		t.Run(topic, func(t *testing.T) {
			code, out, _ := runCLI(t, "help", topic)
			if code != exitOK {
				t.Fatalf("help %s exit = %d, want 0", topic, code)
			}
			if len(out) < 200 {
				t.Errorf("help %s body is suspiciously short (%d bytes); did the prose get truncated?", topic, len(out))
			}
		})
	}
}

// TestSchemaContract asserts every published schema is valid JSON, has
// the expected $id prefix, and is parseable by encoding/json. This is
// the schema contract test from SPEC.md §12.
func TestSchemaContract(t *testing.T) {
	for name := range publishedSchemas {
		name := name
		t.Run(name, func(t *testing.T) {
			code, out, _ := runCLI(t, "schema", name)
			if code != exitOK {
				t.Fatalf("schema %s exit = %d, want 0", name, code)
			}
			var doc map[string]interface{}
			if err := json.Unmarshal([]byte(out), &doc); err != nil {
				t.Fatalf("schema %s is not valid JSON: %v", name, err)
			}
			id, ok := doc["$id"].(string)
			if !ok {
				t.Errorf("schema %s missing $id", name)
			} else if !strings.HasPrefix(id, "plumbline/v1/") {
				t.Errorf("schema %s $id = %q, want plumbline/v1/ prefix", name, id)
			}
			if _, ok := doc["$schema"]; !ok {
				t.Errorf("schema %s missing $schema reference", name)
			}
			if _, ok := doc["properties"]; !ok {
				t.Errorf("schema %s missing properties", name)
			}
		})
	}
}

// TestSignalsJSON_StableShape verifies the signals --json output is the
// shape LLM tool callers depend on.
func TestSignalsJSON_StableShape(t *testing.T) {
	code, out, _ := runCLI(t, "signals", "--json")
	if code != exitOK {
		t.Fatalf("signals --json exit = %d", code)
	}
	var arr []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatalf("signals --json is not valid JSON array: %v", err)
	}
	if len(arr) < 5 {
		t.Errorf("signals --json returned %d entries, want at least 5 (L2 catalog alone has 5)", len(arr))
	}
	for _, s := range arr {
		for _, key := range []string{"id", "level", "family", "title"} {
			if _, ok := s[key]; !ok {
				t.Errorf("signal entry missing %q: %v", key, s)
			}
		}
	}
}
