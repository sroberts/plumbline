// Package config loads and validates .plumbline.yml. The schema is
// fixed in cmd/plumbline/schema.go (`plumbline schema config`); this
// package mirrors it as Go structs with strict (unknown-key-hostile)
// YAML decoding so typos can't silently disable signals.
//
// SPEC.md §6 "Unknown keys are a hard error" is the load-bearing rule
// that this package enforces.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

// DefaultPath is the conventional location of the config file at a
// repo root. Callers can override via --config.
const DefaultPath = ".plumbline.yml"

// Config mirrors the .plumbline.yml schema. Unknown top-level or
// nested keys cause Load to error rather than silently drop them.
type Config struct {
	Profile    string                  `yaml:"profile,omitempty"`
	Thresholds *Thresholds             `yaml:"thresholds,omitempty"`
	Signals    map[string]SignalConfig `yaml:"signals,omitempty"`
	Paths      *Paths                  `yaml:"paths,omitempty"`
}

// Thresholds tunes the pass threshold used to decide whether a level
// is achieved. Defaults live in scoring.DefaultPassThreshold.
type Thresholds struct {
	Pass float64 `yaml:"pass,omitempty"`
}

// SignalConfig is per-signal config, keyed by signal ID in Config.Signals.
type SignalConfig struct {
	Enabled *bool          `yaml:"enabled,omitempty"`
	Args    map[string]any `yaml:"args,omitempty"`
}

// Paths is repo-relative ignore configuration. (Not yet wired into
// scanner — kept here so the YAML round-trips cleanly today; future
// scanner work will pick this up.)
type Paths struct {
	Ignore []string `yaml:"ignore,omitempty"`
}

// LoadDefault looks for the conventional DefaultPath in the given
// directory. Missing file returns (nil, nil) — the config is optional.
func LoadDefault(repoRoot string) (*Config, error) {
	return loadFromPath(joinPath(repoRoot, DefaultPath), false)
}

// Load reads and validates an explicit config path. A missing file at
// an explicit path is an error (the user asked for it). Use LoadDefault
// for the optional default-path lookup.
func Load(path string) (*Config, error) {
	return loadFromPath(path, true)
}

func loadFromPath(path string, requireExist bool) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && !requireExist {
			return nil, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	return Parse(data)
}

func joinPath(dir, base string) string {
	if dir == "" {
		return base
	}
	if dir[len(dir)-1] == '/' {
		return dir + base
	}
	return dir + "/" + base
}

// Parse validates raw YAML bytes against the Config schema. Exposed
// separately so callers (tests, future stdin support) can validate
// content without going through the filesystem.
//
// An empty input returns the zero-value Config, not an error — an
// empty .plumbline.yml is a valid signal that the user has opted in
// but wants all defaults.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if len(bytes.TrimSpace(data)) == 0 {
		return &cfg, nil
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // unknown YAML keys → decode error
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("invalid .plumbline.yml: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Profile != "" {
		switch c.Profile {
		case "default", "go-only", "frontend-only", "oss-cncf":
		default:
			return fmt.Errorf("invalid profile %q (want one of: default, go-only, frontend-only, oss-cncf)", c.Profile)
		}
	}
	if c.Thresholds != nil {
		if c.Thresholds.Pass < 0 || c.Thresholds.Pass > 1 {
			return fmt.Errorf("thresholds.pass = %v out of range [0, 1]", c.Thresholds.Pass)
		}
	}
	return nil
}

// DisabledSignals returns the IDs whose `enabled: false` is set in
// signals:. Convenience for assess.go to merge into ExcludeSignal.
func (c *Config) DisabledSignals() []string {
	if c == nil {
		return nil
	}
	var out []string
	for id, sc := range c.Signals {
		if sc.Enabled != nil && !*sc.Enabled {
			out = append(out, id)
		}
	}
	return out
}
