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

package autoupdate

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"

	"github.com/google/renameio/v2"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
)

const (
	agentUpdateConfigVersion = "v1"
	agentUpdateConfigKind    = "update_config"
)

// AgentUpdateConfig describes the update.yaml file schema.
type AgentUpdateConfig struct {
	// Version of the configuration file
	Version string `yaml:"version"`
	// Kind of configuration file (always "update_config")
	Kind string `yaml:"kind"`
	// Spec contains user-specified configuration.
	Spec AgentUpdateSpec `yaml:"spec"`
	// Status contains state configuration.
	Status AgentUpdateStatus `yaml:"status"`
}

// AgentUpdateSpec describes the spec field in update.yaml.
type AgentUpdateSpec struct {
	// Proxy address
	Proxy string `yaml:"proxy"`
	// Group update identifier
	Group string `yaml:"group"`
	// URLTemplate for the Teleport tgz download URL.
	URLTemplate string `yaml:"url_template"`
	// Enabled controls whether auto-updates are enabled.
	Enabled bool `yaml:"enabled"`
}

// AgentUpdateStatus describes the status field in update.yaml.
type AgentUpdateStatus struct {
	// ActiveVersion is the currently active Teleport version.
	ActiveVersion string `yaml:"active_version"`
}

type AgentUpdater struct {
	Log *slog.Logger
}

// Disable disables agent updates.
// updatePath must be a path to the update.yaml file.
func (u AgentUpdater) Disable(ctx context.Context, updatePath string) error {
	cfg, err := u.readConfig(updatePath)
	if err != nil {
		return trace.Errorf("failed to read updates.yaml: %w", err)
	}
	if !cfg.Spec.Enabled {
		u.Log.InfoContext(ctx, "Automatic updates already disabled")
		return nil
	}
	cfg.Spec.Enabled = false
	if err := u.writeConfig(updatePath, cfg); err != nil {
		return trace.Errorf("failed to write updates.yaml: %w", err)
	}
	return nil
}

// readConfig reads update.yaml
func (AgentUpdater) readConfig(path string) (*AgentUpdateConfig, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &AgentUpdateConfig{
			Version: agentUpdateConfigVersion,
			Kind:    agentUpdateConfigKind,
		}, nil
	}
	if err != nil {
		return nil, trace.Errorf("failed to open: %w", err)
	}
	defer f.Close()
	var cfg AgentUpdateConfig
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, trace.Errorf("failed to parse: %w", err)
	}
	if k := cfg.Kind; k != agentUpdateConfigKind {
		return nil, trace.Errorf("invalid kind %q", k)
	}
	if v := cfg.Version; v != agentUpdateConfigVersion {
		return nil, trace.Errorf("invalid version %q", v)
	}
	return &cfg, nil
}

// writeConfig writes update.yaml atomically, ensuring the file cannot be corrupted.
func (AgentUpdater) writeConfig(filename string, cfg *AgentUpdateConfig) error {
	opts := []renameio.Option{
		renameio.WithPermissions(0755),
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
