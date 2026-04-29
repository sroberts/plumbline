package signals

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestResolveID_KnownAliasRewrites(t *testing.T) {
	got, ok := ResolveID("l2.claude-md")
	if !ok {
		t.Fatal("ResolveID(l2.claude-md) ok=false; want true")
	}
	if got != "l2.agent-instructions" {
		t.Errorf("ResolveID(l2.claude-md) = %q; want l2.agent-instructions", got)
	}
}

func TestResolveID_UnknownPassesThrough(t *testing.T) {
	got, ok := ResolveID("l3.coverage-gate")
	if ok {
		t.Error("ResolveID(l3.coverage-gate) ok=true; want false (not deprecated)")
	}
	if got != "l3.coverage-gate" {
		t.Errorf("ResolveID(l3.coverage-gate) = %q; want passthrough", got)
	}
}

func TestResolveIDs_RewritesMixedSlice(t *testing.T) {
	ResetWarnedForTest()
	in := []string{"l2.claude-md", "l3.coverage-gate", "l2.copilot-instructions"}
	got, fired := ResolveIDs(in, io.Discard)
	want := []string{"l2.agent-instructions", "l3.coverage-gate", "l2.agent-instructions"}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q; want %q", i, got[i], want[i])
		}
	}
	if len(fired) != 2 {
		t.Errorf("fired = %d; want 2", len(fired))
	}
}

func TestResolveIDs_WarnsOnceToWriter(t *testing.T) {
	ResetWarnedForTest()
	var w bytes.Buffer
	// Pass the same deprecated ID twice; warning should fire once.
	_, _ = ResolveIDs([]string{"l2.claude-md", "l2.claude-md"}, &w)
	got := w.String()
	count := strings.Count(got, "deprecated since signal-set")
	if count != 1 {
		t.Errorf("warned %d times; want 1\noutput: %q", count, got)
	}
	if !strings.Contains(got, "l2.claude-md") {
		t.Errorf("warning omits deprecated ID:\n%s", got)
	}
	if !strings.Contains(got, "l2.agent-instructions") {
		t.Errorf("warning omits target ID:\n%s", got)
	}
}

func TestResolveIDs_DiscardSuppressesWarning(t *testing.T) {
	ResetWarnedForTest()
	_, _ = ResolveIDs([]string{"l2.claude-md"}, io.Discard)
	// Pass a real writer next; should still fire (Discard didn't mark warned).
	var w bytes.Buffer
	_, _ = ResolveIDs([]string{"l2.claude-md"}, &w)
	if !strings.Contains(w.String(), "deprecated") {
		t.Errorf("Discard incorrectly marked alias as warned; got %q", w.String())
	}
}

func TestAllAliases_Deterministic(t *testing.T) {
	// Stable order is part of the public contract — `plumbline help
	// compatibility` and verdict JSON output rely on it.
	got1 := AllAliases()
	got2 := AllAliases()
	if len(got1) != len(got2) {
		t.Fatalf("inconsistent length: %d vs %d", len(got1), len(got2))
	}
	for i := range got1 {
		if got1[i].From != got2[i].From {
			t.Errorf("non-deterministic order at i=%d: %q vs %q",
				i, got1[i].From, got2[i].From)
		}
	}
}

func TestLookupAlias_KnownAndUnknown(t *testing.T) {
	a, ok := LookupAlias("l2.claude-md")
	if !ok {
		t.Fatal("LookupAlias(l2.claude-md) ok=false")
	}
	if a.Since == "" || a.Reason == "" {
		t.Errorf("alias missing Since/Reason: %+v", a)
	}
	if _, ok := LookupAlias("nope"); ok {
		t.Error("LookupAlias(nope) should be false")
	}
}
