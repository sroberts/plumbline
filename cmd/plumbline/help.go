package main

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"
)

// helpTopics is the registry of topical help pages. Bodies are stubbed
// in this PR; each topic's prose lands in subsequent milestones (see
// SPEC.md §9.5 topic catalog).
var helpTopics = map[string]string{
	"levels":        "(not implemented in this milestone) The five ACMM levels.",
	"signals":       "(not implemented in this milestone) Signal lifecycle, status, partial-credit semantics.",
	"scoring":       "(not implemented in this milestone) Threshold math, no-skip rule, next_gap.",
	"output":        "(not implemented in this milestone) Every output mode with examples.",
	"config":        "(not implemented in this milestone) The .plumbline.yml schema.",
	"ci":            "(not implemented in this milestone) Wiring plumbline into CI; copy-pasteable YAML.",
	"agents":        "(not implemented in this milestone) Recommended call sequences for LLM tool callers.",
	"profiles":      "(not implemented in this milestone) Named signal presets.",
	"compatibility": "(not implemented in this milestone) Signal-set versions and migration notes.",
}

func newHelpCmd(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "help [topic]",
		Short: "Long-form help on a topic, or the topic index when called bare",
		Long: `plumbline help — long-form, prose help for cross-cutting topics.

Output is plain markdown so an LLM agent can ingest it directly. Each
topic has a stable URL fragment (e.g. 'plumbline help scoring#no-skip-rule')
that error messages can deep-link to.

Without an argument, prints the topic index. With a topic, prints that
topic's full text.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				printTopicIndex(stdout)
				return nil
			}
			topic := args[0]
			body, ok := helpTopics[topic]
			if !ok {
				return errCannotRun(fmt.Errorf("unknown topic: %q. Run 'plumbline help' for the list", topic))
			}
			fmt.Fprintln(stdout, body)
			return nil
		},
	}
	return cmd
}

func printTopicIndex(w io.Writer) {
	keys := make([]string, 0, len(helpTopics))
	for k := range helpTopics {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Fprintln(w, "plumbline help — topical guides")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Topics:")
	for _, k := range keys {
		fmt.Fprintf(w, "  plumbline help %s\n", k)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Run 'plumbline <command> --help' for per-command flag reference.")
}
