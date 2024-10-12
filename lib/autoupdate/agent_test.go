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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
				InsecureSkipVerify: true,
				VersionsDir:        dir,
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
		name    string
		cfg     *AgentUpdateConfig // nil -> file not present
		userCfg AgentUserConfig

		installedVersion  string
		installedTemplate string
		errMatch          string
	}{
		{
			name: "config from file",
			cfg: &AgentUpdateConfig{
				Version: agentUpdateConfigVersion,
				Kind:    agentUpdateConfigKind,
				Spec: AgentUpdateSpec{
					Group:       "group",
					URLTemplate: "template",
				},
				Status: AgentUpdateStatus{
					ActiveVersion: "old",
				},
			},
			installedVersion:  "16.3.0",
			installedTemplate: "template",
		},
		{
			name: "config from user",
			cfg: &AgentUpdateConfig{
				Version: agentUpdateConfigVersion,
				Kind:    agentUpdateConfigKind,
				Spec: AgentUpdateSpec{
					Group:       "old-group",
					URLTemplate: "old-template",
				},
				Status: AgentUpdateStatus{
					ActiveVersion: "old",
				},
			},
			userCfg: AgentUserConfig{
				Group:        "group",
				URLTemplate:  "template",
				ForceVersion: "new",
			},
			installedVersion:  "new",
			installedTemplate: "template",
		},
		{
			name: "already enabled",
			cfg: &AgentUpdateConfig{
				Version: agentUpdateConfigVersion,
				Kind:    agentUpdateConfigKind,
				Spec: AgentUpdateSpec{
					Enabled: true,
				},
				Status: AgentUpdateStatus{
					ActiveVersion: "old",
				},
			},
			installedVersion:  "16.3.0",
			installedTemplate: cdnURITemplate,
		},
		{
			name: "version already installed",
			cfg: &AgentUpdateConfig{
				Version: agentUpdateConfigVersion,
				Kind:    agentUpdateConfigKind,
				Status: AgentUpdateStatus{
					ActiveVersion: "16.3.0",
				},
			},
		},
		{
			name:              "config does not exist",
			installedVersion:  "16.3.0",
			installedTemplate: cdnURITemplate,
		},
		{
			name:     "invalid metadata",
			cfg:      &AgentUpdateConfig{},
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

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// TODO(sclevine): add web API test including group verification
				w.Write([]byte(`{}`))
			}))
			t.Cleanup(server.Close)

			if tt.userCfg.Proxy == "" {
				tt.userCfg.Proxy = strings.TrimPrefix(server.URL, "https://")
			}

			updater, err := NewAgentUpdater(AgentConfig{
				InsecureSkipVerify: true,
				VersionsDir:        dir,
			})
			require.NoError(t, err)

			var installedVersion, installedTemplate string
			updater.Installer = &fakeInstaller{
				FuncInstall: func(_ context.Context, version, template string) error {
					installedVersion = version
					installedTemplate = template
					return nil
				},
			}

			ctx := context.Background()
			err = updater.Enable(ctx, tt.userCfg)
			if tt.errMatch != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.installedVersion, installedVersion)
			require.Equal(t, tt.installedTemplate, installedTemplate)

			data, err := os.ReadFile(cfgPath)
			require.NoError(t, err)
			data = blankTestAddr(data)

			if golden.ShouldSet() {
				golden.Set(t, data)
			}
			require.Equal(t, string(golden.Get(t)), string(data))
		})
	}
}

var serverRegexp = regexp.MustCompile("127.0.0.1:[0-9]+")

func blankTestAddr(s []byte) []byte {
	return serverRegexp.ReplaceAll(s, []byte("localhost"))
}

type fakeInstaller struct {
	FuncInstall func(ctx context.Context, version, template string) error
	FuncRemove  func(ctx context.Context, version string) error
}

func (f *fakeInstaller) Install(ctx context.Context, version, template string) error {
	return f.FuncInstall(ctx, version, template)
}

func (f *fakeInstaller) Remove(ctx context.Context, version string) error {
	return f.FuncRemove(ctx, version)
}
