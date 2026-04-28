package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// Embedded JSON Schemas (draft 2020-12) for plumbline's public output
// types. Hand-written so the schemas exactly match the contract we
// promise in SPEC.md §9; an automated generator would risk drift.
var publishedSchemas = map[string]string{
	"verdict":       schemaVerdict,
	"signal-result": schemaSignalResult,
	"event":         schemaEvent,
	"config":        schemaConfig,
}

var schemaNames = []string{"verdict", "signal-result", "event", "config"}

func newSchemaCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema <name>",
		Short: "Emit the JSON Schema for a public output type",
		Long: `plumbline schema — emit the JSON Schema for a named output type.

Schemas are draft-2020-12. Their $id includes the major version
(plumbline/v1/...). Backwards-incompatible changes bump the major and
ship a deprecation alias for one minor version.

Available names:
  verdict         top-level result of 'assess --json'
  signal-result   one signal's entry within a verdict (also 'inspect --json')
  event           NDJSON event line emitted by '--events ndjson'
  config          .plumbline.yml schema

Examples:
  plumbline schema verdict
  plumbline schema event > event.schema.json

See also:
  plumbline help compatibility   when schemas change between versions
  plumbline help agents          schema-fetching workflow for tool callers`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: schemaNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			body, ok := publishedSchemas[name]
			if !ok {
				return errCannotRun(fmt.Errorf("unknown schema %q (available: verdict, signal-result, event, config)", name))
			}
			fmt.Fprint(stdout, body)
			return nil
		},
	}
	return cmd
}

const schemaVerdict = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "plumbline/v1/verdict",
  "title": "Verdict",
  "type": "object",
  "required": ["schema", "tool_version", "signal_set_version", "ci_system", "repo", "scanned_at", "verdict", "signals"],
  "properties": {
    "schema": { "const": "plumbline/v1" },
    "tool_version": { "type": "string" },
    "signal_set_version": { "type": "string", "examples": ["v1"] },
    "ci_system": { "type": "string", "enum": ["github-actions", "auto"] },
    "repo": { "type": "string", "description": "Absolute path to the scanned repository" },
    "scanned_at": { "type": "string", "format": "date-time" },
    "verdict": {
      "type": "object",
      "required": ["level", "name", "level_scores", "next_gap", "min_confidence_applied"],
      "properties": {
        "level": { "type": "integer", "minimum": 1, "maximum": 5 },
        "name": { "type": "string" },
        "level_scores": {
          "type": "object",
          "patternProperties": { "^[2-5]$": { "type": "number", "minimum": 0, "maximum": 1 } }
        },
        "next_gap": { "type": "array", "items": { "type": "string" } },
        "min_confidence_applied": { "type": "string", "enum": ["low", "medium", "high"] }
      }
    },
    "signals": {
      "type": "array",
      "items": { "$ref": "plumbline/v1/signal-result" }
    }
  }
}
`

const schemaSignalResult = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "plumbline/v1/signal-result",
  "title": "SignalResult",
  "type": "object",
  "required": ["id", "level", "family", "status", "score", "confidence", "method"],
  "properties": {
    "id": { "type": "string", "examples": ["l2.claude-md"] },
    "level": { "type": "integer", "minimum": 1, "maximum": 5 },
    "family": { "type": "string" },
    "title": { "type": "string" },
    "status": { "type": "string", "enum": ["found", "partial", "missing", "na"] },
    "score": { "type": "number", "enum": [0.0, 0.33, 0.67, 1.0] },
    "confidence": { "type": "string", "enum": ["low", "medium", "high"] },
    "method": { "type": "string", "enum": ["filename", "content-regex", "ast", "cross-file"] },
    "evidence": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["path"],
        "properties": {
          "path": { "type": "string" },
          "span": {
            "type": "object",
            "properties": {
              "start": { "type": "integer", "minimum": 1 },
              "end":   { "type": "integer", "minimum": 1 }
            }
          },
          "excerpt": { "type": "string" }
        }
      }
    },
    "notes": { "type": "array", "items": { "type": "string" } },
    "fix_hint": { "type": "string", "description": "Short prose recipe for how to move this signal toward Found." },
    "diag": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["path", "action", "hit"],
        "properties": {
          "path": { "type": "string" },
          "action": { "type": "string", "enum": ["stat", "read", "regex", "ast-query"] },
          "hit": { "type": "boolean" },
          "detail": { "type": "string" }
        }
      }
    }
  }
}
`

const schemaEvent = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "plumbline/v1/event",
  "title": "Event",
  "description": "One NDJSON event line emitted by 'assess --events ndjson' to stderr.",
  "type": "object",
  "required": ["event", "ts"],
  "properties": {
    "event": {
      "type": "string",
      "enum": ["scan.start", "signal.start", "signal.complete", "scan.complete"]
    },
    "ts": { "type": "string", "format": "date-time" },
    "repo": { "type": "string" },
    "signal_count": { "type": "integer", "minimum": 0 },
    "id": { "type": "string" },
    "status": { "type": "string", "enum": ["found", "partial", "missing", "na"] },
    "score": { "type": "number" },
    "duration_ms": { "type": "integer", "minimum": 0 },
    "level": { "type": "integer", "minimum": 1, "maximum": 5 }
  }
}
`

const schemaConfig = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "plumbline/v1/config",
  "title": "Config",
  "description": ".plumbline.yml schema",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "profile": { "type": "string", "enum": ["default", "go-only", "frontend-only", "oss-cncf"] },
    "thresholds": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "pass": { "type": "number", "minimum": 0, "maximum": 1 }
      }
    },
    "signals": {
      "type": "object",
      "patternProperties": {
        "^l[2-5]\\.[a-z0-9-]+$": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "enabled": { "type": "boolean" },
            "args": { "type": "object" }
          }
        }
      }
    },
    "paths": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "ignore": { "type": "array", "items": { "type": "string" } }
      }
    }
  }
}
`
