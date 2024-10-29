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
	"context"
	"encoding/json"
	"errors"
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

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestUpdater_Disable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      *UpdateConfig // nil -> file not present
		errMatch string
	}{
		{
			name: "enabled",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Enabled: true,
				},
			},
		},
		{
			name: "already disabled",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Enabled: false,
				},
			},
		},
		{
			name: "config does not exist",
		},
		{
			name: "invalid metadata",
			cfg: &UpdateConfig{
				Spec: UpdateSpec{
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
			updater, err := NewLocalUpdater(LocalUpdaterConfig{
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

func TestUpdater_Enable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cfg        *UpdateConfig // nil -> file not present
		userCfg    OverrideConfig
		installErr error
		flags      InstallFlags

		removedVersion    string
		installedVersion  string
		installedTemplate string
		requestGroup      string
		errMatch          string
	}{
		{
			name: "config from file",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Group:       "group",
					URLTemplate: "https://example.com",
				},
				Status: UpdateStatus{
					ActiveVersion: "old-version",
				},
			},
			installedVersion:  "16.3.0",
			installedTemplate: "https://example.com",
			requestGroup:      "group",
		},
		{
			name: "config from user",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Group:       "old-group",
					URLTemplate: "https://example.com/old",
				},
				Status: UpdateStatus{
					ActiveVersion: "old-version",
				},
			},
			userCfg: OverrideConfig{
				Group:        "new-group",
				URLTemplate:  "https://example.com/new",
				ForceVersion: "new-version",
			},
			installedVersion:  "new-version",
			installedTemplate: "https://example.com/new",
		},
		{
			name: "already enabled",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Enabled: true,
				},
				Status: UpdateStatus{
					ActiveVersion: "old-version",
				},
			},
			installedVersion:  "16.3.0",
			installedTemplate: cdnURITemplate,
		},
		{
			name: "insecure URL",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					URLTemplate: "http://example.com",
				},
			},
			errMatch: "URL must use TLS",
		},
		{
			name: "install error",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					URLTemplate: "https://example.com",
				},
			},
			installErr: errors.New("install error"),
			errMatch:   "install error",
		},
		{
			name: "version already installed",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Status: UpdateStatus{
					ActiveVersion: "16.3.0",
				},
			},
			installedVersion:  "16.3.0",
			installedTemplate: cdnURITemplate,
		},
		{
			name: "backup version removed on install",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Status: UpdateStatus{
					ActiveVersion: "old-version",
					BackupVersion: "backup-version",
				},
			},
			installedVersion:  "16.3.0",
			installedTemplate: cdnURITemplate,
			removedVersion:    "backup-version",
		},
		{
			name: "backup version kept for validation",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Status: UpdateStatus{
					ActiveVersion: "16.3.0",
					BackupVersion: "backup-version",
				},
			},
			installedVersion:  "16.3.0",
			installedTemplate: cdnURITemplate,
			removedVersion:    "",
		},
		{
			name:              "config does not exist",
			installedVersion:  "16.3.0",
			installedTemplate: cdnURITemplate,
		},
		{
			name:              "FIPS and Enterprise flags",
			flags:             FlagEnterprise | FlagFIPS,
			installedVersion:  "16.3.0",
			installedTemplate: cdnURITemplate,
		},
		{
			name:     "invalid metadata",
			cfg:      &UpdateConfig{},
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

			var requestedGroup string
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestedGroup = r.URL.Query().Get("group")
				config := webclient.PingResponse{
					AutoUpdate: webclient.AutoUpdateSettings{
						AgentVersion: "16.3.0",
					},
				}
				if tt.flags&FlagEnterprise != 0 {
					config.Edition = "ent"
				}
				config.FIPS = tt.flags&FlagFIPS != 0
				err := json.NewEncoder(w).Encode(config)
				require.NoError(t, err)
			}))
			t.Cleanup(server.Close)

			if tt.userCfg.Proxy == "" {
				tt.userCfg.Proxy = strings.TrimPrefix(server.URL, "https://")
			}

			updater, err := NewLocalUpdater(LocalUpdaterConfig{
				InsecureSkipVerify: true,
				VersionsDir:        dir,
			})
			require.NoError(t, err)

			var (
				installedVersion  string
				installedTemplate string
				linkedVersion     string
				removedVersion    string
				installedFlags    InstallFlags
			)
			updater.Installer = &testInstaller{
				FuncInstall: func(_ context.Context, version, template string, flags InstallFlags) error {
					installedVersion = version
					installedTemplate = template
					installedFlags = flags
					return tt.installErr
				},
				FuncLink: func(_ context.Context, version string) error {
					linkedVersion = version
					return nil
				},
				FuncList: func(_ context.Context) (versions []string, err error) {
					return []string{"old"}, nil
				},
				FuncRemove: func(_ context.Context, version string) error {
					removedVersion = version
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
			require.Equal(t, tt.installedVersion, linkedVersion)
			require.Equal(t, tt.removedVersion, removedVersion)
			require.Equal(t, tt.flags, installedFlags)
			require.Equal(t, tt.requestGroup, requestedGroup)

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

type testInstaller struct {
	FuncInstall func(ctx context.Context, version, template string, flags InstallFlags) error
	FuncRemove  func(ctx context.Context, version string) error
	FuncLink    func(ctx context.Context, version string) error
	FuncList    func(ctx context.Context) (versions []string, err error)
}

func (ti *testInstaller) Install(ctx context.Context, version, template string, flags InstallFlags) error {
	return ti.FuncInstall(ctx, version, template, flags)
}

func (ti *testInstaller) Remove(ctx context.Context, version string) error {
	return ti.FuncRemove(ctx, version)
}

func (ti *testInstaller) Link(ctx context.Context, version string) error {
	return ti.FuncLink(ctx, version)
}

func (ti *testInstaller) List(ctx context.Context) (versions []string, err error) {
	return ti.FuncList(ctx)
}
