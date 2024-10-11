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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/utils/golden"
)

func TestAgentUpdater_Disable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      *AgentUpdateConfig // nil -> file not present
		errMatch string
	}{
		{
			name: "enabled",
			cfg: &AgentUpdateConfig{
				Version: agentUpdateConfigVersion,
				Kind:    agentUpdateConfigKind,
				Spec: AgentUpdateSpec{
					Enabled: true,
				},
			},
		},
		{
			name: "already disabled",
			cfg: &AgentUpdateConfig{
				Version: agentUpdateConfigVersion,
				Kind:    agentUpdateConfigKind,
				Spec: AgentUpdateSpec{
					Enabled: false,
				},
			},
		},
		{
			name: "config does not exist",
		},
		{
			name: "invalid metadata",
			cfg: &AgentUpdateConfig{
				Spec: AgentUpdateSpec{
					Enabled: true,
				},
			},
			errMatch: "invalid",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "update.yaml")

			// Create config file only if provided in test case
			if tt.cfg != nil {
				b, err := yaml.Marshal(tt.cfg)
				require.NoError(t, err)
				err = os.WriteFile(cfgPath, b, 0600)
				require.NoError(t, err)
			}
			updater, err := NewAgentUpdater(AgentConfig{
				DownloadInsecure: true,
				VersionsDir:      dir,
			})
			require.NoError(t, err)
			err = updater.Disable(context.Background())
			if tt.errMatch != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
				return
			}
			require.NoError(t, err)

			data, err := os.ReadFile(cfgPath)

			// If no config is present, disable should not create it
			if tt.cfg == nil {
				require.ErrorIs(t, err, os.ErrNotExist)
				return
			}
			require.NoError(t, err)

			if golden.ShouldSet() {
				golden.Set(t, data)
			}
			require.Equal(t, string(golden.Get(t)), string(data))
		})
	}
}

func TestAgentUpdater_Enable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      *AgentUpdateConfig // nil -> file not present
		userCfg  AgentUserConfig
		errMatch string
	}{
		{
			name: "defaults",
			cfg: &AgentUpdateConfig{
				Version: agentUpdateConfigVersion,
				Kind:    agentUpdateConfigKind,
				Spec:    AgentUpdateSpec{},
			},
		},
		{
			name: "user-provided values",
			cfg: &AgentUpdateConfig{
				Version: agentUpdateConfigVersion,
				Kind:    agentUpdateConfigKind,
				Spec:    AgentUpdateSpec{},
			},
		},
		{
			name: "already enabled",
			cfg: &AgentUpdateConfig{
				Version: agentUpdateConfigVersion,
				Kind:    agentUpdateConfigKind,
				Spec: AgentUpdateSpec{
					Enabled: true,
				},
			},
		},
		{
			name: "install forced version",
			cfg: &AgentUpdateConfig{
				Version: agentUpdateConfigVersion,
				Kind:    agentUpdateConfigKind,
				Spec: AgentUpdateSpec{
					Enabled: true,
				},
			},
		},
		{
			name: "install cluster version",
			cfg: &AgentUpdateConfig{
				Version: agentUpdateConfigVersion,
				Kind:    agentUpdateConfigKind,
				Spec: AgentUpdateSpec{
					Enabled: true,
				},
			},
		},
		{
			name: "version already installed",
			cfg: &AgentUpdateConfig{
				Version: agentUpdateConfigVersion,
				Kind:    agentUpdateConfigKind,
				Spec: AgentUpdateSpec{
					Enabled: true,
				},
			},
		},
		{
			name: "config does not exist",
		},
		{
			name: "invalid metadata",
			cfg: &AgentUpdateConfig{
				Spec: AgentUpdateSpec{
					Enabled: true,
				},
			},
			errMatch: "invalid",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "update.yaml")

			// Create config file only if provided in test case
			if tt.cfg != nil {
				b, err := yaml.Marshal(tt.cfg)
				require.NoError(t, err)
				err = os.WriteFile(cfgPath, b, 0600)
				require.NoError(t, err)
			}

			updater, err := NewAgentUpdater(AgentConfig{
				DownloadInsecure: true,
				VersionsDir:      dir,
			})
			require.NoError(t, err)
			updater.Installer = &fakeInstaller{}

			err = updater.Enable(context.Background(), tt.userCfg)
			if tt.errMatch != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
				return
			}
			require.NoError(t, err)

			data, err := os.ReadFile(cfgPath)

			// If no config is present, disable should not create it
			if tt.cfg == nil {
				require.ErrorIs(t, err, os.ErrNotExist)
				return
			}
			require.NoError(t, err)

			if golden.ShouldSet() {
				golden.Set(t, data)
			}
			require.Equal(t, string(golden.Get(t)), string(data))
		})
	}
}

type fakeInstaller struct{}

func (fakeInstaller) Install(ctx context.Context, version, template string) error {
	return nil
}

func (fakeInstaller) Remove(ctx context.Context, version string) error {
	return nil
}
