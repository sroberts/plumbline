package signals_test

import (
	"testing"

	"github.com/sroberts/plumbline/internal/signals"

	// Side-effect imports register every signal with signals.Default.
	// We need them here because aliases.To: must point at a signal
	// that actually exists; without these imports the Default registry
	// is empty and the integrity check below would falsely fail.
	_ "github.com/sroberts/plumbline/internal/signals/l2"
	_ "github.com/sroberts/plumbline/internal/signals/l3"
	_ "github.com/sroberts/plumbline/internal/signals/l4"
	_ "github.com/sroberts/plumbline/internal/signals/l5"
)

func TestAllAliases_EveryTargetExists(t *testing.T) {
	// An alias whose To: points at a non-existent signal would
	// silently rewrite to a dead end. This integration test runs
	// against the real registry (populated via init()) so a typo in
	// aliases.go fails CI rather than at runtime.
	for _, a := range signals.AllAliases() {
		if _, ok := signals.Default.Get(a.To); !ok {
			t.Errorf("alias %q -> %q: target not registered in Default registry",
				a.From, a.To)
		}
	}
}
