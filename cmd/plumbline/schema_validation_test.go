// Real schema-conformance tests. The existing TestSchemaContract only
// verifies the schema documents themselves are well-formed; this file
// runs plumbline against fixture repos and validates the actual JSON
// output against the published JSON Schemas. Drift between schema and
// code now fails CI instead of going undetected.
package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// loadSchemaCompiler returns a compiler with all published schemas
// pre-registered under stable test-only URIs. The verdict schema's
// signals[].$ref to signal-result is inlined at compile time —
// sidesteps the relative-$ref resolution dance without touching the
// public $id form (which is part of the v0.x compatibility surface).
func loadSchemaCompiler(t *testing.T) *jsonschema.Compiler {
	t.Helper()
	c := jsonschema.NewCompiler()

	// Parse signal-result first so we can splice it into verdict.
	var signalResultDoc any
	if err := json.Unmarshal([]byte(publishedSchemas["signal-result"]), &signalResultDoc); err != nil {
		t.Fatalf("signal-result parse: %v", err)
	}

	for name, raw := range publishedSchemas {
		var doc any
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			t.Fatalf("schema %s parse: %v", name, err)
		}
		// Inline signal-result wherever the verdict schema $refs it.
		if name == "verdict" {
			obj := doc.(map[string]any)
			props := obj["properties"].(map[string]any)
			signals := props["signals"].(map[string]any)
			signals["items"] = signalResultDoc
		}
		key := "test://plumbline/" + name
		if err := c.AddResource(key, doc); err != nil {
			t.Fatalf("AddResource %s: %v", name, err)
		}
	}
	return c
}

func compile(t *testing.T, c *jsonschema.Compiler, id string) *jsonschema.Schema {
	t.Helper()
	s, err := c.Compile(id)
	if err != nil {
		t.Fatalf("compile %s: %v", id, err)
	}
	return s
}

func validate(t *testing.T, schema *jsonschema.Schema, body []byte) {
	t.Helper()
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("body is not JSON: %v\n%s", err, body)
	}
	if err := schema.Validate(doc); err != nil {
		t.Errorf("validation failed against schema:\n%v\n--- body ---\n%s", err, body)
	}
}

func TestSchemaConformance_AssessJSON(t *testing.T) {
	c := loadSchemaCompiler(t)
	verdictSchema := compile(t, c, "test://plumbline/verdict")

	dir := t.TempDir()
	// Substantive enough to populate signals + next_gap.
	writeFile(t, dir, "CLAUDE.md", "# CLAUDE\n\n"+strings.Repeat("rule.\n", 25))

	code, out, _ := runCLI(t, "assess", "--json", dir)
	if code != exitOK {
		t.Fatalf("assess exit = %d", code)
	}
	validate(t, verdictSchema, []byte(out))
}

// The reproducible snapshot normalizes scanned_at and repo; this makes
// sure those normalized values still satisfy the verdict schema (both
// fields are required — scanned_at as date-time).
func TestSchemaConformance_SnapshotJSON(t *testing.T) {
	c := loadSchemaCompiler(t)
	verdictSchema := compile(t, c, "test://plumbline/verdict")

	dir := t.TempDir()
	writeFile(t, dir, "CLAUDE.md", "# CLAUDE\n\n"+strings.Repeat("rule.\n", 25))

	out := filepath.Join(dir, "snap.json")
	code, _, errOut := runCLI(t, "snapshot", "--format", "json", "--out", out, dir)
	if code != exitOK {
		t.Fatalf("snapshot exit = %d (stderr: %s)", code, errOut)
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	validate(t, verdictSchema, body)
}

func TestSchemaConformance_InspectJSON(t *testing.T) {
	c := loadSchemaCompiler(t)
	signalResultSchema := compile(t, c, "test://plumbline/signal-result")

	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r")
	code, out, _ := runCLI(t, "inspect", "l2.agent-instructions", dir, "--json")
	if code != exitOK {
		t.Fatalf("inspect exit = %d", code)
	}
	validate(t, signalResultSchema, []byte(out))
}

func TestSchemaConformance_EventStream(t *testing.T) {
	c := loadSchemaCompiler(t)
	eventSchema := compile(t, c, "test://plumbline/event")

	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# r")

	// Capture stderr (where events go) by writing the report to a file.
	out := filepath.Join(dir, "verdict.json")
	t.Setenv("HOME", t.TempDir())
	stdout, stderr := captureToFiles(t, dir, out)
	defer cleanup(stdout, stderr)
	_ = runCLIIntoFiles(t, stdout, stderr, "assess", "--json", "--events", "ndjson", dir)

	stderrBytes, err := os.ReadFile(stderr)
	if err != nil {
		t.Fatal(err)
	}
	lineCount := 0
	for _, line := range strings.Split(strings.TrimSpace(string(stderrBytes)), "\n") {
		if line == "" {
			continue
		}
		validate(t, eventSchema, []byte(line))
		lineCount++
	}
	if lineCount == 0 {
		t.Errorf("expected at least one NDJSON event line; got none")
	}
}

// captureToFiles allocates two unique paths inside dir for stdout/stderr.
func captureToFiles(t *testing.T, dir, stdoutPath string) (string, string) {
	t.Helper()
	if stdoutPath == "" {
		stdoutPath = filepath.Join(dir, "stdout.txt")
	}
	stderrPath := filepath.Join(dir, "stderr.txt")
	return stdoutPath, stderrPath
}

func cleanup(_, _ string) {}

// runCLIIntoFiles is like runCLI but writes stdout / stderr to the
// given paths. Returns the exit code.
func runCLIIntoFiles(t *testing.T, stdoutPath, stderrPath string, args ...string) int {
	t.Helper()
	stdout, err := os.Create(stdoutPath)
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	stderr, err := os.Create(stderrPath)
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	return run(args, stdout, stderr)
}

// TestSchemaConformance_VersionJSON keeps the version output honest.
// Not currently covered by a schema — track if we add one.
func TestSchemaConformance_VersionJSON(t *testing.T) {
	_, out, _ := runCLI(t, "version", "--json")
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("version --json is not valid JSON: %v", err)
	}
	for _, key := range []string{"version", "commit", "signal_set_version", "schema"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("version --json missing %q key", key)
		}
	}
}

// Confirm the validator catches drift.
func TestSchemaConformance_RejectsBadOutput(t *testing.T) {
	c := loadSchemaCompiler(t)
	signalResultSchema := compile(t, c, "test://plumbline/signal-result")

	bogus := []byte(`{"id":"l2.x","level":2,"family":"x","status":"INVALID","score":0,"confidence":"low","method":"filename"}`)
	var doc any
	_ = json.Unmarshal(bogus, &doc)
	if err := signalResultSchema.Validate(doc); err == nil {
		t.Error("validator accepted a signal-result with invalid status; the validator is broken")
	}
}

// silence unused-import nag if context import drifts.
var _ = context.Background
