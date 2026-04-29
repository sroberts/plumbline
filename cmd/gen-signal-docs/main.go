// gen-signal-docs writes docs/SIGNALS.md from the registered signal
// catalog. It's the source-of-truth generator: editing SIGNALS.md by
// hand will be undone by the next regeneration. The PR workflow at
// .github/workflows/docs-signals.yml runs this and opens a follow-up
// PR when output drifts from what's checked in.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/sroberts/plumbline/internal/signals"
	"github.com/sroberts/plumbline/pkg/acmm"

	// Side-effect imports register every signal with signals.Default.
	_ "github.com/sroberts/plumbline/internal/signals/l2"
	_ "github.com/sroberts/plumbline/internal/signals/l3"
	_ "github.com/sroberts/plumbline/internal/signals/l4"
	_ "github.com/sroberts/plumbline/internal/signals/l5"
)

func main() {
	out := flag.String("out", "docs/SIGNALS.md", "Output path. \"-\" = stdout.")
	flag.Parse()

	body := render(signals.Default.All())

	if *out == "-" {
		_, err := os.Stdout.Write([]byte(body))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := os.WriteFile(*out, []byte(body), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func render(sigs []signals.Signal) string {
	byLevel := map[acmm.Level][]signals.Signal{}
	for _, s := range sigs {
		byLevel[s.Level()] = append(byLevel[s.Level()], s)
	}

	var b strings.Builder
	b.WriteString("# Signal Catalog\n\n")
	b.WriteString("_Auto-generated from `signals.Default` by `cmd/gen-signal-docs`. Do not edit by hand — the regeneration workflow at `.github/workflows/docs-signals.yml` opens a PR when this file drifts from the source._\n\n")
	b.WriteString("Each signal is a deterministic detector that returns a status (`found` / `partial` / `missing` / `na`) plus a confidence and method. The full Result schema is at `plumbline schema signal-result`.\n\n")

	for _, lvl := range []acmm.Level{
		acmm.LevelInstructed,
		acmm.LevelMeasured,
		acmm.LevelAdaptive,
		acmm.LevelSelfSustaining,
	} {
		levelSigs := byLevel[lvl]
		if len(levelSigs) == 0 {
			continue
		}
		sort.Slice(levelSigs, func(i, j int) bool { return levelSigs[i].ID() < levelSigs[j].ID() })

		fmt.Fprintf(&b, "## L%d — %s\n\n", lvl, lvl.Name())
		fmt.Fprintln(&b, "| ID | Family | Title |")
		fmt.Fprintln(&b, "|---|---|---|")
		for _, s := range levelSigs {
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", s.ID(), s.Family(), escapeMD(s.Title()))
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "_%d signals total._\n", len(sigs))
	return b.String()
}

// escapeMD escapes pipe characters so titles containing them don't
// break the table layout.
func escapeMD(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}
