package acmm

import (
	"context"
	"testing"
)

// Recording must cost nothing unless --debug asked for it. Detectors call
// Probe on every branch, so an always-on recorder would allocate on the
// normal scan path — which is what the perf budgets in
// internal/signals guard against.
func TestDiagnosticsDisabledByDefault(t *testing.T) {
	d := NewDiagnostics(context.Background())
	if got := d.Probe("a.yml", "stat", true); !got {
		t.Error("Probe must return hit unchanged when disabled")
	}
	r := d.Attach(Result{Status: StatusFound})
	if r.Diag != nil {
		t.Errorf("disabled recorder attached %d entries, want none", len(r.Diag))
	}
}

func TestDiagnosticsRecordsWhenEnabled(t *testing.T) {
	ctx := WithDiagnostics(context.Background())
	if !DiagEnabled(ctx) {
		t.Fatal("WithDiagnostics did not mark the context")
	}
	d := NewDiagnostics(ctx)
	d.Probe("codecov.yml", "stat", false)
	d.Probe("ci.yml", "threshold flag", true, "step: Coverage gate")

	r := d.Attach(Result{Status: StatusFound})
	// Two probes plus the trailing result entry.
	if len(r.Diag) != 3 {
		t.Fatalf("got %d diag entries, want 3: %+v", len(r.Diag), r.Diag)
	}
	if r.Diag[0].Hit || !r.Diag[1].Hit {
		t.Error("hit flags not recorded faithfully")
	}
	if r.Diag[1].Detail != "step: Coverage gate" {
		t.Errorf("detail = %q", r.Diag[1].Detail)
	}
	if last := r.Diag[2]; last.Action != "result" || last.Detail != string(StatusFound) {
		t.Errorf("trailing result entry = %+v", last)
	}
}

// A nil recorder is usable, so a detector need not branch on whether it
// built one.
func TestDiagnosticsNilSafe(t *testing.T) {
	var d *Diagnostics
	if got := d.Probe("a", "b", true); !got {
		t.Error("nil Probe must pass hit through")
	}
	if r := d.Attach(Result{Status: StatusMissing}); r.Diag != nil {
		t.Error("nil Attach must not populate Diag")
	}
}
