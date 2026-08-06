package skill

import (
	"strings"
	"testing"

	"github.com/sroberts/plumbline/internal/signals"

	// Registration is by init(), so the level packages have to be linked in
	// for signals.Default to be populated.
	_ "github.com/sroberts/plumbline/internal/signals/l2"
	_ "github.com/sroberts/plumbline/internal/signals/l3"
	_ "github.com/sroberts/plumbline/internal/signals/l4"
	_ "github.com/sroberts/plumbline/internal/signals/l5"
)

// The skill body hand-lists signal IDs under "Stable contracts", and that
// list is what an agent reads to know what plumbline can detect. It has no
// compiler relationship to the registry, so adding a signal silently
// leaves it stale — which is how l3.metrics-acted-on was missing from it.
//
// docs/SIGNALS.md is regenerated and drift-gated in CI. This list is not
// generated (Body is a const, consumed as one), so this test is its gate.
func TestSkillBodyListsEverySignal(t *testing.T) {
	var missing []string
	for _, s := range signals.Default.All() {
		// The body lists IDs without their level prefix under a per-level
		// heading: "l3.coverage-gate" appears as "coverage-gate".
		id := s.ID()
		short := id
		if i := strings.IndexByte(id, '.'); i >= 0 {
			short = id[i+1:]
		}
		if !strings.Contains(corePlumblineGuide, short) {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		t.Errorf("signals registered but absent from the skill body's stable-contracts list: %v\n"+
			"add them to corePlumblineGuide in skill.go", missing)
	}
}
