package signals

import (
	"context"
	"slices"
	"testing"

	"github.com/sroberts/plumbline/internal/scanner"
	"github.com/sroberts/plumbline/pkg/acmm"
)

// fakeSignal is a minimal Signal used to drive registry tests.
type fakeSignal struct {
	id     string
	level  acmm.Level
	family string
	title  string
	result acmm.Result
}

func (f *fakeSignal) ID() string                                                 { return f.id }
func (f *fakeSignal) Level() acmm.Level                                          { return f.level }
func (f *fakeSignal) Family() string                                             { return f.family }
func (f *fakeSignal) Title() string                                              { return f.title }
func (f *fakeSignal) Detect(_ context.Context, _ *scanner.RepoIndex) acmm.Result { return f.result }

func idsOf(sigs []Signal) []string {
	out := make([]string, len(sigs))
	for i, s := range sigs {
		out[i] = s.ID()
	}
	return out
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	s := &fakeSignal{id: "l2.test", level: acmm.LevelInstructed, family: "test"}
	r.Register(s)

	got, ok := r.Get("l2.test")
	if !ok {
		t.Fatal("Get(l2.test): not found")
	}
	if got.ID() != "l2.test" {
		t.Errorf("Get returned signal with ID %q, want l2.test", got.ID())
	}

	_, ok = r.Get("missing")
	if ok {
		t.Error("Get(missing): expected not found")
	}
}

func TestRegistry_DuplicateIDPanics(t *testing.T) {
	r := NewRegistry()
	s := &fakeSignal{id: "l2.dup", level: acmm.LevelInstructed, family: "test"}
	r.Register(s)

	defer func() {
		if recover() == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()
	r.Register(s)
}

func TestRegistry_AllReturnsDeterministicOrder(t *testing.T) {
	r := NewRegistry()
	// Register in non-sorted order. All() must sort by (level, id).
	r.Register(&fakeSignal{id: "l3.b", level: acmm.LevelMeasured, family: "y"})
	r.Register(&fakeSignal{id: "l2.a", level: acmm.LevelInstructed, family: "x"})
	r.Register(&fakeSignal{id: "l3.a", level: acmm.LevelMeasured, family: "x"})
	r.Register(&fakeSignal{id: "l4.a", level: acmm.LevelAdaptive, family: "z"})

	got := idsOf(r.All())
	want := []string{"l2.a", "l3.a", "l3.b", "l4.a"}
	if !slices.Equal(got, want) {
		t.Errorf("All() = %v, want %v", got, want)
	}
}

func TestRegistry_AtLevel(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeSignal{id: "l2.a", level: acmm.LevelInstructed, family: "x"})
	r.Register(&fakeSignal{id: "l3.b", level: acmm.LevelMeasured, family: "y"})
	r.Register(&fakeSignal{id: "l3.c", level: acmm.LevelMeasured, family: "y"})

	got := idsOf(r.AtLevel(acmm.LevelMeasured))
	want := []string{"l3.b", "l3.c"}
	if !slices.Equal(got, want) {
		t.Errorf("AtLevel(L3) = %v, want %v", got, want)
	}

	got = idsOf(r.AtLevel(acmm.LevelAdaptive))
	if len(got) != 0 {
		t.Errorf("AtLevel(L4) = %v, want empty", got)
	}
}

func TestRegistry_InFamily(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeSignal{id: "l2.a", level: acmm.LevelInstructed, family: "instructions"})
	r.Register(&fakeSignal{id: "l2.b", level: acmm.LevelInstructed, family: "templates"})
	r.Register(&fakeSignal{id: "l3.c", level: acmm.LevelMeasured, family: "instructions"})

	got := idsOf(r.InFamily("instructions"))
	want := []string{"l2.a", "l3.c"}
	if !slices.Equal(got, want) {
		t.Errorf("InFamily(instructions) = %v, want %v", got, want)
	}
}

func TestRegistry_NewIsEmpty(t *testing.T) {
	r := NewRegistry()
	if len(r.All()) != 0 {
		t.Errorf("fresh registry has %d signals, want 0", len(r.All()))
	}
}
