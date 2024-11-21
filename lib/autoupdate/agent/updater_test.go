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
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, VersionsDirName, "update.yaml")

			updater, err := NewLocalUpdater(LocalUpdaterConfig{
				InsecureSkipVerify: true,
				DataDir:            dir,
			})
			require.NoError(t, err)

			// Create config file only if provided in test case
			if tt.cfg != nil {
				b, err := yaml.Marshal(tt.cfg)
				require.NoError(t, err)
				err = os.WriteFile(cfgPath, b, 0600)
				require.NoError(t, err)
			}

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

func TestUpdater_Unpin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      *UpdateConfig // nil -> file not present
		errMatch string
	}{
		{
			name: "pinned",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Pinned: true,
				},
			},
		},
		{
			name: "not pinned",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Pinned: false,
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
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, VersionsDirName, "update.yaml")

			updater, err := NewLocalUpdater(LocalUpdaterConfig{
				InsecureSkipVerify: true,
				DataDir:            dir,
			})
			require.NoError(t, err)

			// Create config file only if provided in test case
			if tt.cfg != nil {
				b, err := yaml.Marshal(tt.cfg)
				require.NoError(t, err)
				err = os.WriteFile(cfgPath, b, 0600)
				require.NoError(t, err)
			}

			err = updater.Unpin(context.Background())
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

func TestUpdater_Update(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cfg        *UpdateConfig // nil -> file not present
		flags      InstallFlags
		inWindow   bool
		installErr error
		setupErr   error
		reloadErr  error

		removedVersion    string
		installedVersion  string
		installedTemplate string
		linkedVersion     string
		requestGroup      string
		reloadCalls       int
		revertCalls       int
		setupCalls        int
		errMatch          string
	}{
		{
			name: "updates enabled during window",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Group:       "group",
					URLTemplate: "https://example.com",
					Enabled:     true,
				},
				Status: UpdateStatus{
					ActiveVersion: "old-version",
				},
			},
			inWindow: true,

			installedVersion:  "16.3.0",
			installedTemplate: "https://example.com",
			linkedVersion:     "16.3.0",
			requestGroup:      "group",
			reloadCalls:       1,
			setupCalls:        1,
		},
		{
			name: "updates disabled during window",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Group:       "group",
					URLTemplate: "https://example.com",
					Enabled:     false,
				},
				Status: UpdateStatus{
					ActiveVersion: "old-version",
				},
			},
			inWindow: true,
		},
		{
			name: "updates enabled outside of window",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Group:       "group",
					URLTemplate: "https://example.com",
					Enabled:     true,
				},
				Status: UpdateStatus{
					ActiveVersion: "old-version",
				},
			},
			requestGroup: "group",
		},
		{
			name: "updates disabled outside of window",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Group:       "group",
					URLTemplate: "https://example.com",
					Enabled:     false,
				},
				Status: UpdateStatus{
					ActiveVersion: "old-version",
				},
			},
		},
		{
			name: "insecure URL",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					URLTemplate: "http://example.com",
					Enabled:     true,
				},
			},
			inWindow: true,

			errMatch: "URL must use TLS",
		},
		{
			name: "install error",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Enabled: true,
				},
			},
			inWindow:   true,
			installErr: errors.New("install error"),

			installedVersion:  "16.3.0",
			installedTemplate: cdnURITemplate,
			errMatch:          "install error",
		},
		{
			name: "version already installed in window",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					URLTemplate: "https://example.com",
					Enabled:     true,
				},
				Status: UpdateStatus{
					ActiveVersion: "16.3.0",
				},
			},
			inWindow: true,
		},
		{
			name: "version already installed outside of window",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					URLTemplate: "https://example.com",
					Enabled:     true,
				},
				Status: UpdateStatus{
					ActiveVersion: "16.3.0",
				},
			},
		},
		{
			name: "backup version removed on install",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					URLTemplate: "https://example.com",
					Enabled:     true,
				},
				Status: UpdateStatus{
					ActiveVersion: "old-version",
					BackupVersion: "backup-version",
				},
			},
			inWindow: true,

			installedVersion:  "16.3.0",
			installedTemplate: "https://example.com",
			linkedVersion:     "16.3.0",
			removedVersion:    "backup-version",
			reloadCalls:       1,
			setupCalls:        1,
		},
		{
			name: "backup version kept when no change",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					URLTemplate: "https://example.com",
					Enabled:     true,
				},
				Status: UpdateStatus{
					ActiveVersion: "16.3.0",
					BackupVersion: "backup-version",
				},
			},
			inWindow: true,
		},
		{
			name: "config does not exist",
		},
		{
			name: "FIPS and Enterprise flags",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					URLTemplate: "https://example.com",
					Enabled:     true,
				},
				Status: UpdateStatus{
					ActiveVersion: "old-version",
					BackupVersion: "backup-version",
				},
			},
			inWindow: true,
			flags:    FlagEnterprise | FlagFIPS,

			installedVersion:  "16.3.0",
			installedTemplate: "https://example.com",
			linkedVersion:     "16.3.0",
			removedVersion:    "backup-version",
			reloadCalls:       1,
			setupCalls:        1,
		},
		{
			name:     "invalid metadata",
			cfg:      &UpdateConfig{},
			errMatch: "invalid",
		},
		{
			name: "setup fails",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					URLTemplate: "https://example.com",
					Enabled:     true,
				},
				Status: UpdateStatus{
					ActiveVersion: "old-version",
					BackupVersion: "backup-version",
				},
			},
			inWindow: true,
			setupErr: errors.New("setup error"),

			installedVersion:  "16.3.0",
			installedTemplate: "https://example.com",
			linkedVersion:     "16.3.0",
			removedVersion:    "backup-version",
			reloadCalls:       0,
			revertCalls:       1,
			setupCalls:        1,
			errMatch:          "setup error",
		},
		{
			name: "reload fails",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					URLTemplate: "https://example.com",
					Enabled:     true,
				},
				Status: UpdateStatus{
					ActiveVersion: "old-version",
					BackupVersion: "backup-version",
				},
			},
			inWindow:  true,
			reloadErr: errors.New("reload error"),

			installedVersion:  "16.3.0",
			installedTemplate: "https://example.com",
			linkedVersion:     "16.3.0",
			removedVersion:    "backup-version",
			reloadCalls:       2,
			revertCalls:       1,
			setupCalls:        1,
			errMatch:          "reload error",
		},
		{
			name: "skip version",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					URLTemplate: "https://example.com",
					Enabled:     true,
				},
				Status: UpdateStatus{
					ActiveVersion: "old-version",
					BackupVersion: "backup-version",
					SkipVersion:   "16.3.0",
				},
			},
			inWindow: true,
		},
		{
			name: "pinned version",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					URLTemplate: "https://example.com",
					Enabled:     true,
					Pinned:      true,
				},
				Status: UpdateStatus{
					ActiveVersion: "old-version",
					BackupVersion: "backup-version",
				},
			},
			inWindow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestedGroup string
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestedGroup = r.URL.Query().Get("group")
				config := webclient.PingResponse{
					AutoUpdate: webclient.AutoUpdateSettings{
						AgentVersion:    "16.3.0",
						AgentAutoUpdate: tt.inWindow,
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

			dir := t.TempDir()
			cfgPath := filepath.Join(dir, VersionsDirName, "update.yaml")

			updater, err := NewLocalUpdater(LocalUpdaterConfig{
				InsecureSkipVerify: true,
				DataDir:            dir,
			})
			require.NoError(t, err)

			// Create config file only if provided in test case
			if tt.cfg != nil {
				tt.cfg.Spec.Proxy = strings.TrimPrefix(server.URL, "https://")
				b, err := yaml.Marshal(tt.cfg)
				require.NoError(t, err)
				err = os.WriteFile(cfgPath, b, 0600)
				require.NoError(t, err)
			}

			var (
				installedVersion  string
				installedTemplate string
				linkedVersion     string
				removedVersion    string
				installedFlags    InstallFlags
				revertFuncCalls   int
				setupCalls        int
				revertSetupCalls  int
				reloadCalls       int
			)
			updater.Installer = &testInstaller{
				FuncInstall: func(_ context.Context, version, template string, flags InstallFlags) error {
					installedVersion = version
					installedTemplate = template
					installedFlags = flags
					return tt.installErr
				},
				FuncLink: func(_ context.Context, version string) (revert func(context.Context) bool, err error) {
					linkedVersion = version
					return func(_ context.Context) bool {
						revertFuncCalls++
						return true
					}, nil
				},
				FuncList: func(_ context.Context) (versions []string, err error) {
					return []string{"old"}, nil
				},
				FuncRemove: func(_ context.Context, version string) error {
					removedVersion = version
					return nil
				},
			}
			updater.Process = &testProcess{
				FuncReload: func(_ context.Context) error {
					reloadCalls++
					return tt.reloadErr
				},
			}
			updater.Setup = func(_ context.Context) error {
				setupCalls++
				return tt.setupErr
			}
			updater.Revert = func(_ context.Context) error {
				revertSetupCalls++
				return nil
			}

			ctx := context.Background()
			err = updater.Update(ctx)
			if tt.errMatch != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.installedVersion, installedVersion)
			require.Equal(t, tt.installedTemplate, installedTemplate)
			require.Equal(t, tt.linkedVersion, linkedVersion)
			require.Equal(t, tt.removedVersion, removedVersion)
			require.Equal(t, tt.flags, installedFlags)
			require.Equal(t, tt.requestGroup, requestedGroup)
			require.Equal(t, tt.reloadCalls, reloadCalls)
			require.Equal(t, tt.revertCalls, revertSetupCalls)
			require.Equal(t, tt.revertCalls, revertFuncCalls)
			require.Equal(t, tt.setupCalls, setupCalls)

			if tt.cfg == nil {
				_, err := os.Stat(cfgPath)
				require.Error(t, err)
				return
			}

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

func TestUpdater_LinkPackage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		cfg              *UpdateConfig // nil -> file not present
		tryLinkSystemErr error

		syncCalls          int
		tryLinkSystemCalls int
		errMatch           string
	}{
		{
			name: "updates enabled",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Enabled: true,
				},
			},

			tryLinkSystemCalls: 0,
			syncCalls:          0,
		},
		{
			name: "updates disabled",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Enabled: false,
				},
			},

			tryLinkSystemCalls: 1,
			syncCalls:          1,
		},
		{
			name: "already linked",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Enabled: false,
				},
			},
			tryLinkSystemErr: ErrLinked,

			tryLinkSystemCalls: 1,
			syncCalls:          0,
		},
		{
			name: "link error",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Enabled: false,
				},
			},
			tryLinkSystemErr: errors.New("bad"),

			tryLinkSystemCalls: 1,
			syncCalls:          0,
			errMatch:           "bad",
		},
		{
			name:               "no config",
			tryLinkSystemCalls: 1,
			syncCalls:          1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, VersionsDirName, "update.yaml")

			updater, err := NewLocalUpdater(LocalUpdaterConfig{
				InsecureSkipVerify: true,
				DataDir:            dir,
			})
			require.NoError(t, err)

			// Create config file only if provided in test case
			if tt.cfg != nil {
				b, err := yaml.Marshal(tt.cfg)
				require.NoError(t, err)
				err = os.WriteFile(cfgPath, b, 0600)
				require.NoError(t, err)
			}

			var tryLinkSystemCalls int
			updater.Installer = &testInstaller{
				FuncTryLinkSystem: func(_ context.Context) error {
					tryLinkSystemCalls++
					return tt.tryLinkSystemErr
				},
			}
			var syncCalls int
			updater.Process = &testProcess{
				FuncSync: func(_ context.Context) error {
					syncCalls++
					return nil
				},
			}

			ctx := context.Background()
			err = updater.LinkPackage(ctx)
			if tt.errMatch != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.tryLinkSystemCalls, tryLinkSystemCalls)
			require.Equal(t, tt.syncCalls, syncCalls)
		})
	}
}

func TestUpdater_Install(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cfg        *UpdateConfig // nil -> file not present
		userCfg    OverrideConfig
		flags      InstallFlags
		installErr error
		setupErr   error
		reloadErr  error

		removedVersion    string
		installedVersion  string
		installedTemplate string
		linkedVersion     string
		requestGroup      string
		reloadCalls       int
		revertCalls       int
		setupCalls        int
		errMatch          string
	}{
		{
			name: "config from file",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Enabled:     true,
					Group:       "group",
					URLTemplate: "https://example.com",
				},
				Status: UpdateStatus{
					ActiveVersion: "old-version",
				},
			},

			installedVersion:  "16.3.0",
			installedTemplate: "https://example.com",
			linkedVersion:     "16.3.0",
			requestGroup:      "group",
			reloadCalls:       1,
			setupCalls:        1,
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
				UpdateSpec: UpdateSpec{
					Enabled:     true,
					Group:       "new-group",
					URLTemplate: "https://example.com/new",
				},
				ForceVersion: "new-version",
			},

			installedVersion:  "new-version",
			installedTemplate: "https://example.com/new",
			linkedVersion:     "new-version",
			requestGroup:      "new-group",
			reloadCalls:       1,
			setupCalls:        1,
		},
		{
			name: "defaults",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Status: UpdateStatus{
					ActiveVersion: "old-version",
				},
			},

			installedVersion:  "16.3.0",
			installedTemplate: cdnURITemplate,
			linkedVersion:     "16.3.0",
			reloadCalls:       1,
			setupCalls:        1,
		},
		{
			name: "override skip",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Status: UpdateStatus{
					ActiveVersion: "old-version",
					SkipVersion:   "16.3.0",
				},
			},

			installedVersion:  "16.3.0",
			installedTemplate: cdnURITemplate,
			linkedVersion:     "16.3.0",
			reloadCalls:       1,
			setupCalls:        1,
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
			},
			installErr: errors.New("install error"),

			installedVersion:  "16.3.0",
			linkedVersion:     "",
			installedTemplate: cdnURITemplate,
			errMatch:          "install error",
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
			linkedVersion:     "16.3.0",
			reloadCalls:       0,
			setupCalls:        1,
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
			linkedVersion:     "16.3.0",
			removedVersion:    "backup-version",
			reloadCalls:       1,
			setupCalls:        1,
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
			linkedVersion:     "16.3.0",
			removedVersion:    "",
			reloadCalls:       0,
			setupCalls:        1,
		},
		{
			name: "config does not exist",

			installedVersion:  "16.3.0",
			installedTemplate: cdnURITemplate,
			linkedVersion:     "16.3.0",
			reloadCalls:       1,
			setupCalls:        1,
		},
		{
			name:              "FIPS and Enterprise flags",
			flags:             FlagEnterprise | FlagFIPS,
			installedVersion:  "16.3.0",
			installedTemplate: cdnURITemplate,
			linkedVersion:     "16.3.0",
			reloadCalls:       1,
			setupCalls:        1,
		},
		{
			name:     "invalid metadata",
			cfg:      &UpdateConfig{},
			errMatch: "invalid",
		},
		{
			name:     "setup fails",
			setupErr: errors.New("setup error"),

			installedVersion:  "16.3.0",
			installedTemplate: cdnURITemplate,
			linkedVersion:     "16.3.0",
			reloadCalls:       0,
			revertCalls:       1,
			setupCalls:        1,
			errMatch:          "setup error",
		},
		{
			name:      "reload fails",
			reloadErr: errors.New("reload error"),

			installedVersion:  "16.3.0",
			installedTemplate: cdnURITemplate,
			linkedVersion:     "16.3.0",
			reloadCalls:       2,
			revertCalls:       1,
			setupCalls:        1,
			errMatch:          "reload error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, VersionsDirName, "update.yaml")

			updater, err := NewLocalUpdater(LocalUpdaterConfig{
				InsecureSkipVerify: true,
				DataDir:            dir,
			})
			require.NoError(t, err)

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

			var (
				installedVersion  string
				installedTemplate string
				linkedVersion     string
				removedVersion    string
				installedFlags    InstallFlags
				revertFuncCalls   int
				reloadCalls       int
				setupCalls        int
				revertSetupCalls  int
			)
			updater.Installer = &testInstaller{
				FuncInstall: func(_ context.Context, version, template string, flags InstallFlags) error {
					installedVersion = version
					installedTemplate = template
					installedFlags = flags
					return tt.installErr
				},
				FuncLink: func(_ context.Context, version string) (revert func(context.Context) bool, err error) {
					linkedVersion = version
					return func(_ context.Context) bool {
						revertFuncCalls++
						return true
					}, nil
				},
				FuncList: func(_ context.Context) (versions []string, err error) {
					return []string{"old"}, nil
				},
				FuncRemove: func(_ context.Context, version string) error {
					removedVersion = version
					return nil
				},
			}
			updater.Process = &testProcess{
				FuncReload: func(_ context.Context) error {
					reloadCalls++
					return tt.reloadErr
				},
			}
			updater.Setup = func(_ context.Context) error {
				setupCalls++
				return tt.setupErr
			}
			updater.Revert = func(_ context.Context) error {
				revertSetupCalls++
				return nil
			}

			ctx := context.Background()
			err = updater.Install(ctx, tt.userCfg)
			if tt.errMatch != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.installedVersion, installedVersion)
			require.Equal(t, tt.installedTemplate, installedTemplate)
			require.Equal(t, tt.linkedVersion, linkedVersion)
			require.Equal(t, tt.removedVersion, removedVersion)
			require.Equal(t, tt.flags, installedFlags)
			require.Equal(t, tt.requestGroup, requestedGroup)
			require.Equal(t, tt.reloadCalls, reloadCalls)
			require.Equal(t, tt.revertCalls, revertSetupCalls)
			require.Equal(t, tt.revertCalls, revertFuncCalls)
			require.Equal(t, tt.setupCalls, setupCalls)

			if tt.cfg == nil && err != nil {
				_, err := os.Stat(cfgPath)
				require.Error(t, err)
				return
			}

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
	FuncInstall       func(ctx context.Context, version, template string, flags InstallFlags) error
	FuncRemove        func(ctx context.Context, version string) error
	FuncLink          func(ctx context.Context, version string) (revert func(context.Context) bool, err error)
	FuncLinkSystem    func(ctx context.Context) (revert func(context.Context) bool, err error)
	FuncTryLink       func(ctx context.Context, version string) error
	FuncTryLinkSystem func(ctx context.Context) error
	FuncUnlinkSystem  func(ctx context.Context) error
	FuncList          func(ctx context.Context) (versions []string, err error)
}

func (ti *testInstaller) Install(ctx context.Context, version, template string, flags InstallFlags) error {
	return ti.FuncInstall(ctx, version, template, flags)
}

func (ti *testInstaller) Remove(ctx context.Context, version string) error {
	return ti.FuncRemove(ctx, version)
}

func (ti *testInstaller) Link(ctx context.Context, version string) (revert func(context.Context) bool, err error) {
	return ti.FuncLink(ctx, version)
}

func (ti *testInstaller) LinkSystem(ctx context.Context) (revert func(context.Context) bool, err error) {
	return ti.FuncLinkSystem(ctx)
}

func (ti *testInstaller) TryLink(ctx context.Context, version string) error {
	return ti.FuncTryLink(ctx, version)
}

func (ti *testInstaller) TryLinkSystem(ctx context.Context) error {
	return ti.FuncTryLinkSystem(ctx)
}

func (ti *testInstaller) UnlinkSystem(ctx context.Context) error {
	return ti.FuncUnlinkSystem(ctx)
}

func (ti *testInstaller) List(ctx context.Context) (versions []string, err error) {
	return ti.FuncList(ctx)
}

type testProcess struct {
	FuncReload func(ctx context.Context) error
	FuncSync   func(ctx context.Context) error
}

func (tp *testProcess) Reload(ctx context.Context) error {
	return tp.FuncReload(ctx)
}

func (tp *testProcess) Sync(ctx context.Context) error {
	return tp.FuncSync(ctx)
}
