package main

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"

	"github.com/spf13/cobra"

	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// signalDescriptor is the public shape of `signals --json` and the
// per-signal entry consumed by LLM tool callers.
type signalDescriptor struct {
	ID     string     `json:"id"`
	Level  acmm.Level `json:"level"`
	Family string     `json:"family"`
	Title  string     `json:"title"`
}

func newSignalsCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		asJSON       bool
		levelFilters []int
		familyFilter []string
	)

	cmd := &cobra.Command{
		Use:   "signals",
		Short: "List every signal the tool knows how to detect",
		Long: `plumbline signals — list every signal the tool knows how to detect.

Static — does not scan a repo. Filterable by ACMM level and family. The
JSON output is the canonical way for an LLM agent to discover what's
detectable before calling 'assess'.

Examples:
  # All signals, grouped by level.
  plumbline signals

  # Only L3 signals in the coverage family.
  plumbline signals --level 3 --family coverage

  # Machine-readable for tool callers.
  plumbline signals --json

See also:
  plumbline explain <id>     long-form description of one signal
  plumbline help signals     signal lifecycle and partial-credit semantics`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			descriptors := buildDescriptors(levelFilters, familyFilter)

			if asJSON {
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(descriptors)
			}

			printSignalsText(stdout, descriptors)
			return nil
		},
	}

	f := cmd.Flags()
	f.BoolVar(&asJSON, "json", false, "Emit the registry as JSON to stdout.")
	f.IntSliceVar(&levelFilters, "level", nil, "Only list signals at level N (2-5). Repeatable.")
	f.StringSliceVar(&familyFilter, "family", nil, "Only list signals in family <name>. Repeatable.")
	return cmd
}

func buildDescriptors(levelFilters []int, familyFilters []string) []signalDescriptor {
	all := signals.Default.All()
	desc := make([]signalDescriptor, 0, len(all))
	for _, s := range all {
		if len(levelFilters) > 0 && !slices.Contains(levelFilters, int(s.Level())) {
			continue
		}
		if len(familyFilters) > 0 && !slices.Contains(familyFilters, s.Family()) {
			continue
		}
		desc = append(desc, signalDescriptor{
			ID:     s.ID(),
			Level:  s.Level(),
			Family: s.Family(),
			Title:  s.Title(),
		})
	}
	return desc
}

func printSignalsText(w io.Writer, descriptors []signalDescriptor) {
	currentLevel := acmm.Level(0)
	for _, d := range descriptors {
		if d.Level != currentLevel {
			if currentLevel != 0 {
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "Level %d (%s)\n", d.Level, d.Level.Name())
			currentLevel = d.Level
		}
		fmt.Fprintf(w, "  %-30s %-14s %s\n", d.ID, d.Family, d.Title)
	}
}
