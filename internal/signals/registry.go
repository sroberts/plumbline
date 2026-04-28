// Package signals defines the Signal interface and the registry that
// holds every detector plumbline knows about. Concrete signals live in
// subpackages (l2, l3, l4, l5) and register themselves via init().
package signals

import (
	"context"
	"fmt"
	"slices"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// Signal is the unit of pluggability. One file per signal under
// internal/signals/lN/. See SPEC.md §6.
type Signal interface {
	ID() string
	Level() acmm.Level
	Family() string
	Title() string
	Detect(ctx context.Context, idx *scanner.RepoIndex) acmm.Result
}

// Registry holds the set of known signals. The CLI uses Default for
// production scans; tests construct fresh registries with NewRegistry.
type Registry struct {
	byID map[string]Signal
}

// NewRegistry returns an empty Registry suitable for tests and for
// alternate signal sets.
func NewRegistry() *Registry {
	return &Registry{byID: make(map[string]Signal)}
}

// Register adds s to the registry. Panics on duplicate ID — signal IDs
// are part of the public contract and collisions indicate a programming
// error rather than something to recover from at runtime.
func (r *Registry) Register(s Signal) {
	id := s.ID()
	if _, exists := r.byID[id]; exists {
		panic(fmt.Sprintf("signals: duplicate registration for %q", id))
	}
	r.byID[id] = s
}

// Get returns the signal with the given ID, or false if not registered.
func (r *Registry) Get(id string) (Signal, bool) {
	s, ok := r.byID[id]
	return s, ok
}

// All returns every registered signal in deterministic (level, id) order.
func (r *Registry) All() []Signal {
	out := make([]Signal, 0, len(r.byID))
	for _, s := range r.byID {
		out = append(out, s)
	}
	sortByLevelAndID(out)
	return out
}

// AtLevel returns signals at the given ACMM level, in deterministic order.
func (r *Registry) AtLevel(l acmm.Level) []Signal {
	out := make([]Signal, 0)
	for _, s := range r.byID {
		if s.Level() == l {
			out = append(out, s)
		}
	}
	sortByLevelAndID(out)
	return out
}

// InFamily returns signals in the given family, in deterministic order.
func (r *Registry) InFamily(name string) []Signal {
	out := make([]Signal, 0)
	for _, s := range r.byID {
		if s.Family() == name {
			out = append(out, s)
		}
	}
	sortByLevelAndID(out)
	return out
}

func sortByLevelAndID(sigs []Signal) {
	slices.SortFunc(sigs, func(a, b Signal) int {
		if a.Level() != b.Level() {
			return int(a.Level()) - int(b.Level())
		}
		switch {
		case a.ID() < b.ID():
			return -1
		case a.ID() > b.ID():
			return 1
		}
		return 0
	})
}

// Default is the package-level singleton populated by signal subpackages
// at init() time. The CLI uses this for production scans; tests should
// prefer NewRegistry() to avoid global state.
var Default = NewRegistry()
