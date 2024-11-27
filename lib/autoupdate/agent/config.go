/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package agent

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
)

const (
	// updateConfigName specifies the name of the file inside versionsDirName containing configuration for the teleport update.
	updateConfigName = "update.yaml"

	// UpdateConfig metadata
	updateConfigVersion = "v1"
	updateConfigKind    = "update_config"
)

// UpdateConfig describes the update.yaml file schema.
type UpdateConfig struct {
	// Version of the configuration file
	Version string `yaml:"version"`
	// Kind of configuration file (always "update_config")
	Kind string `yaml:"kind"`
	// Spec contains user-specified configuration.
	Spec UpdateSpec `yaml:"spec"`
	// Status contains state configuration.
	Status UpdateStatus `yaml:"status"`
}

// UpdateSpec describes the spec field in update.yaml.
type UpdateSpec struct {
	// Proxy address
	Proxy string `yaml:"proxy"`
	// Group specifies the update group identifier for the agent.
	Group string `yaml:"group,omitempty"`
	// URLTemplate for the Teleport tgz download URL.
	URLTemplate string `yaml:"url_template,omitempty"`
	// Enabled controls whether auto-updates are enabled.
	Enabled bool `yaml:"enabled"`
	// Pinned controls whether the active_version is pinned.
	Pinned bool `yaml:"pinned"`
}

// UpdateStatus describes the status field in update.yaml.
type UpdateStatus struct {
	// ActiveVersion is the currently active Teleport version.
	ActiveVersion string `yaml:"active_version"`
	// BackupVersion is the last working version of Teleport.
	BackupVersion string `yaml:"backup_version"`
	// SkipVersion is the last reverted version of Teleport.
	SkipVersion string `yaml:"skip_version,omitempty"`
}

// readConfig reads UpdateConfig from a file.
func readConfig(path string) (*UpdateConfig, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &UpdateConfig{
			Version: updateConfigVersion,
			Kind:    updateConfigKind,
		}, nil
	}
	if err != nil {
		return nil, trace.Errorf("failed to open: %w", err)
	}
	defer f.Close()
	var cfg UpdateConfig
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, trace.Errorf("failed to parse: %w", err)
	}
	if k := cfg.Kind; k != updateConfigKind {
		return nil, trace.Errorf("invalid kind %q", k)
	}
	if v := cfg.Version; v != updateConfigVersion {
		return nil, trace.Errorf("invalid version %q", v)
	}
	return &cfg, nil
}

// writeConfig writes UpdateConfig to a file atomically, ensuring the file cannot be corrupted.
func writeConfig(filename string, cfg *UpdateConfig) error {
	opts := []renameio.Option{
		renameio.WithPermissions(configFileMode),
		renameio.WithExistingPermissions(),
	}
	t, err := renameio.NewPendingFile(filename, opts...)
	if err != nil {
		return trace.Wrap(err)
	}
	defer t.Cleanup()
	err = yaml.NewEncoder(t).Encode(cfg)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(t.CloseAtomicallyReplace())
}

func validateConfigSpec(spec *UpdateSpec, override OverrideConfig) error {
	if override.Proxy != "" {
		spec.Proxy = override.Proxy
	}
	if override.Group != "" {
		spec.Group = override.Group
	}
	switch override.URLTemplate {
	case "":
	case "default":
		spec.URLTemplate = ""
	default:
		spec.URLTemplate = override.URLTemplate
	}
	if spec.URLTemplate != "" &&
		!strings.HasPrefix(strings.ToLower(spec.URLTemplate), "https://") {
		return trace.Errorf("Teleport download URL must use TLS (https://)")
	}
	if override.Enabled {
		spec.Enabled = true
	}
	if override.Pinned {
		spec.Pinned = true
	}
	return nil
}

// Status of the agent auto-updates system.
type Status struct {
	UpdateSpec   `yaml:",inline"`
	UpdateStatus `yaml:",inline"`
	FindResp     `yaml:",inline"`
}

// FindResp summarizes the auto-update status response from cluster.
type FindResp struct {
	// Version of Teleport to install
	TargetVersion string `yaml:"target_version"`
	// Flags describing the edition of Teleport
	Flags InstallFlags `yaml:"flags"`
	// InWindow is true when the install should happen now.
	InWindow bool `yaml:"in_window"`
	// Jitter duration before an automated install
	Jitter time.Duration `yaml:"jitter"`
}

// InstallFlags sets flags for the Teleport installation
type InstallFlags int

const (
	// FlagEnterprise installs enterprise Teleport
	FlagEnterprise InstallFlags = 1 << iota
	// FlagFIPS installs FIPS Teleport
	FlagFIPS
)

func (i InstallFlags) MarshalYAML() (any, error) {
	return i.Strings(), nil
}

func (i InstallFlags) Strings() []string {
	var out []string
	for _, flag := range []InstallFlags{
		FlagEnterprise,
		FlagFIPS,
	} {
		if i&flag != 0 {
			out = append(out, flag.String())
		}
	}
	return out
}

func (i InstallFlags) String() string {
	switch i {
	case 0:
		return ""
	case FlagEnterprise:
		return "Enterprise"
	case FlagFIPS:
		return "FIPS"
	}
	return "Unknown"
}
