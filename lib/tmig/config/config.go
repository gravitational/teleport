// Package config implements loading and validation of tmig run configuration files.
package config

import (
	"fmt"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the top-level tmig run configuration.
type Config struct {
	Target       TargetConfig `yaml:"target"`
	Migrations   []Migration  `yaml:"migrations"`
	Concurrency  int          `yaml:"concurrency"`
	MarkerPrefix string       `yaml:"marker_prefix"`
}

// TargetConfig describes the destination cluster/scope.
type TargetConfig struct {
	Proxy    string `yaml:"proxy"`
	Identity string `yaml:"identity"`
}

// Migration describes a single source cluster and its scope mappings.
type Migration struct {
	Source   SourceConfig `yaml:"source"`
	SSH      SSHConfig    `yaml:"ssh"`
	Mappings []Mapping    `yaml:"mappings"`
}

// SourceConfig describes the source cluster to migrate agents from.
type SourceConfig struct {
	Proxy    string `yaml:"proxy"`
	Identity string `yaml:"identity"`
}

// SSHConfig holds SSH access parameters for the source cluster's agents.
type SSHConfig struct {
	Login string `yaml:"login"`
}

// Mapping describes how agents matching a selector are assigned to a target scope.
type Mapping struct {
	Selector      map[string]string `yaml:"selector"`
	Scope         string            `yaml:"scope"`
	InstallSuffix string            `yaml:"install_suffix"`
}

// Load reads and validates a tmig run configuration file at the given path.
func Load(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := &Config{
		Concurrency:  32,
		MarkerPrefix: "tmig.teleport.dev",
	}

	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Target.Proxy == "" {
		return fmt.Errorf("target.proxy is required")
	}
	for i, m := range c.Migrations {
		if m.Source.Proxy == "" {
			return fmt.Errorf("migrations[%d].source.proxy is required", i)
		}
		suffixes := make(map[string]bool)
		for j, mp := range m.Mappings {
			if len(mp.Selector) == 0 {
				return fmt.Errorf("migrations[%d].mappings[%d]: selector is required", i, j)
			}
			suffix := mp.InstallSuffix
			if suffix == "" {
				suffix = deriveSuffix(mp.Scope)
			}
			if suffixes[suffix] {
				return fmt.Errorf("migrations[%d]: duplicate install_suffix %q", i, suffix)
			}
			suffixes[suffix] = true
		}
	}
	return nil
}

func (c *Config) applyDefaults() {
	for i, m := range c.Migrations {
		for j, mp := range m.Mappings {
			if mp.InstallSuffix == "" {
				c.Migrations[i].Mappings[j].InstallSuffix = deriveSuffix(mp.Scope)
			}
		}
	}
}

func deriveSuffix(scope string) string {
	if scope == "" {
		return "tmig"
	}
	parts := strings.Split(strings.Trim(scope, "/"), "/")
	result := strings.Join(parts, "-")
	return strings.ReplaceAll(result, "_", "-")
}

// EffectiveYAML returns the merged effective config as YAML for --print-config.
func (c *Config) EffectiveYAML() ([]byte, error) {
	return yaml.Marshal(c)
}

// IsUnscopedTarget returns true if this mapping has no scope.
func (m *Mapping) IsUnscopedTarget() bool {
	return m.Scope == ""
}

// ScopeLeaf returns the last path component of the scope.
func (m *Mapping) ScopeLeaf() string {
	return path.Base(m.Scope)
}
