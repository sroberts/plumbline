package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse_ValidConfig(t *testing.T) {
	in := []byte(`
profile: go-only
thresholds:
  pass: 0.8
signals:
  l3.user-feedback:
    enabled: false
  l3.coverage-gate:
    enabled: true
paths:
  ignore:
    - vendor/
`)
	cfg, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.Profile != "go-only" {
		t.Errorf("Profile = %q", cfg.Profile)
	}
	if cfg.Thresholds == nil || cfg.Thresholds.Pass != 0.8 {
		t.Errorf("Thresholds.Pass = %+v", cfg.Thresholds)
	}
	if len(cfg.Signals) != 2 {
		t.Errorf("len(Signals) = %d, want 2", len(cfg.Signals))
	}
}

func TestParse_UnknownTopLevelKeyIsError(t *testing.T) {
	// Per SPEC.md §6: "Unknown keys are a hard error — typos shouldn't
	// silently disable signals." This is the load-bearing assertion.
	in := []byte(`profile: default
typo_at_top_level: oops
`)
	_, err := Parse(in)
	if err == nil {
		t.Fatal("expected error on unknown key, got nil")
	}
	if !strings.Contains(err.Error(), "typo_at_top_level") {
		t.Errorf("error should name the typo; got: %v", err)
	}
}

func TestParse_UnknownSignalKeyIsError(t *testing.T) {
	in := []byte(`signals:
  l3.coverage-gate:
    enabled: true
    unknown_arg: bad
`)
	_, err := Parse(in)
	if err == nil {
		t.Fatal("expected error on unknown signal-config key")
	}
}

func TestParse_InvalidProfileRejected(t *testing.T) {
	in := []byte(`profile: invented-profile`)
	_, err := Parse(in)
	if err == nil {
		t.Fatal("expected error on invalid profile")
	}
	if !strings.Contains(err.Error(), "invented-profile") {
		t.Errorf("error should name the bad profile; got: %v", err)
	}
}

func TestParse_ThresholdOutOfRangeRejected(t *testing.T) {
	in := []byte(`thresholds:
  pass: 1.5
`)
	_, err := Parse(in)
	if err == nil {
		t.Fatal("expected error on out-of-range threshold")
	}
}

func TestParse_EmptyConfigOK(t *testing.T) {
	cfg, err := Parse([]byte(""))
	if err != nil {
		t.Fatalf("empty config should parse, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil cfg for empty input")
	}
}

func TestLoad_MissingFileIsErrorWhenExplicit(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.yml")
	_, err := Load(missing)
	if err == nil {
		t.Error("Load(missing) should error — explicit --config means user expects the file")
	}
}

func TestLoadDefault_MissingFileIsNotErrorWhenImplicit(t *testing.T) {
	dir := t.TempDir() // no .plumbline.yml in here
	cfg, err := LoadDefault(dir)
	if err != nil {
		t.Errorf("LoadDefault on dir without config should not error; got %v", err)
	}
	if cfg != nil {
		t.Errorf("LoadDefault on dir without config should return nil cfg; got %+v", cfg)
	}
}

func TestLoadDefault_PicksUpFileAtRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, DefaultPath),
		[]byte("profile: default\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadDefault(dir)
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	if cfg == nil || cfg.Profile != "default" {
		t.Errorf("got %+v, want Profile=default", cfg)
	}
}

func TestDisabledSignals_ReturnsOnlyExplicitlyDisabled(t *testing.T) {
	enabled := true
	disabled := false
	cfg := &Config{Signals: map[string]SignalConfig{
		"l3.user-feedback":  {Enabled: &disabled},
		"l3.coverage-gate":  {Enabled: &enabled},
		"l4.flake-recovery": {}, // unset → not "disabled"
	}}
	got := cfg.DisabledSignals()
	if len(got) != 1 || got[0] != "l3.user-feedback" {
		t.Errorf("DisabledSignals = %v, want [l3.user-feedback]", got)
	}
}

func TestDisabledSignals_NilReceiver(t *testing.T) {
	var cfg *Config
	if got := cfg.DisabledSignals(); got != nil {
		t.Errorf("nil cfg.DisabledSignals = %v, want nil", got)
	}
}
