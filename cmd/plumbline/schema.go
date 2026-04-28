package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// schemaNames is the set of public output types whose JSON Schemas
// plumbline publishes. See SPEC.md §9.5.
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
			fmt.Fprintf(stderr, "(M1: schema %q recognized; publishing lands with the JSON output PR.)\n", name)
			return errCannotRun(errors.New("not implemented in this milestone"))
		},
	}
	return cmd
}
