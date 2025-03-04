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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/lib/autoupdate"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestWarnUmask(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		old  int
		warn bool
	}{
		{old: 0o000, warn: false},
		{old: 0o001, warn: true},
		{old: 0o011, warn: true},
		{old: 0o111, warn: true},
		{old: 0o002, warn: false},
		{old: 0o020, warn: false},
		{old: 0o022, warn: false},
		{old: 0o220, warn: true},
		{old: 0o200, warn: true},
		{old: 0o222, warn: true},
		{old: 0o333, warn: true},
		{old: 0o444, warn: true},
		{old: 0o555, warn: true},
		{old: 0o666, warn: true},
		{old: 0o777, warn: true},
	} {
		t.Run(fmt.Sprintf("old umask %o", tt.old), func(t *testing.T) {
			ctx := context.Background()
			out := &bytes.Buffer{}
			warnUmask(ctx, slog.New(slog.NewTextHandler(out,
				&slog.HandlerOptions{ReplaceAttr: msgOnly},
			)), tt.old)
			assert.Equal(t, tt.warn, strings.Contains(out.String(), "detected"))
		})
	}
}

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
			ns := &Namespace{installDir: dir}
			_, err := ns.Init()
			require.NoError(t, err)
			cfgPath := filepath.Join(ns.Dir(), updateConfigName)
			ctx := context.Background()
			updater, err := NewLocalUpdater(ctx, LocalUpdaterConfig{
				InsecureSkipVerify: true,
			}, ns)
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
			ns := &Namespace{installDir: dir}
			_, err := ns.Init()
			require.NoError(t, err)
			cfgPath := filepath.Join(ns.Dir(), updateConfigName)
			ctx := context.Background()
			updater, err := NewLocalUpdater(ctx, LocalUpdaterConfig{
				InsecureSkipVerify: true,
			}, ns)
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
		name            string
		cfg             *UpdateConfig // nil -> file not present
		flags           autoupdate.InstallFlags
		inWindow        bool
		now             bool
		agpl            bool
		installErr      error
		setupErr        error
		reloadErr       error
		notActive       bool
		notEnabled      bool
		linkedRevisions []Revision

		removedRevisions  []Revision
		installedRevision Revision
		installedBaseURL  string
		linkedRevision    Revision
		requestGroup      string
		reloadCalls       int
		revertCalls       int
		setupCalls        int
		restarted         bool
		errMatch          string
	}{
		{
			name: "updates enabled during window",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path:    defaultPathDir,
					Group:   "group",
					BaseURL: "https://example.com",
					Enabled: true,
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
				},
			},
			inWindow: true,

			removedRevisions:  []Revision{NewRevision("unknown-version", 0)},
			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  "https://example.com",
			linkedRevision:    NewRevision("16.3.0", 0),
			requestGroup:      "group",
			setupCalls:        1,
			restarted:         true,
		},
		{
			name: "updates enabled now",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path:    defaultPathDir,
					Group:   "group",
					BaseURL: "https://example.com",
					Enabled: true,
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
				},
			},
			now: true,

			removedRevisions:  []Revision{NewRevision("unknown-version", 0)},
			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  "https://example.com",
			linkedRevision:    NewRevision("16.3.0", 0),
			requestGroup:      "group",
			setupCalls:        1,
			restarted:         true,
		},
		{
			name: "updates enabled now, not started or enabled",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path:    defaultPathDir,
					Group:   "group",
					BaseURL: "https://example.com",
					Enabled: true,
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
				},
			},
			now:        true,
			notEnabled: true,
			notActive:  true,

			removedRevisions:  []Revision{NewRevision("unknown-version", 0)},
			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  "https://example.com",
			linkedRevision:    NewRevision("16.3.0", 0),
			requestGroup:      "group",
			setupCalls:        1,
			restarted:         true,
		},
		{
			name: "updates disabled during window",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path:    defaultPathDir,
					Group:   "group",
					BaseURL: "https://example.com",
					Enabled: false,
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
				},
			},
			inWindow: true,
		},
		{
			name: "missing path during window",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Group:   "group",
					BaseURL: "https://example.com",
					Enabled: true,
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
				},
			},
			inWindow: true,
			errMatch: "destination path",
		},
		{
			name: "updates enabled outside of window",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path:    defaultPathDir,
					Group:   "group",
					BaseURL: "https://example.com",
					Enabled: true,
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
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
					Path:    defaultPathDir,
					Group:   "group",
					BaseURL: "https://example.com",
					Enabled: false,
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
				},
			},
		},
		{
			name: "insecure URL",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path:    defaultPathDir,
					BaseURL: "http://example.com",
					Enabled: true,
				},
			},
			inWindow: true,

			errMatch: "must use TLS",
		},
		{
			name: "install error",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path:    defaultPathDir,
					Enabled: true,
				},
			},
			inWindow:   true,
			installErr: errors.New("install error"),

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  autoupdate.DefaultBaseURL,
			errMatch:          "install error",
		},
		{
			name: "version already installed in window",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path:    defaultPathDir,
					BaseURL: "https://example.com",
					Enabled: true,
				},
				Status: UpdateStatus{
					Active: NewRevision("16.3.0", 0),
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
					Path:    defaultPathDir,
					BaseURL: "https://example.com",
					Enabled: true,
				},
				Status: UpdateStatus{
					Active: NewRevision("16.3.0", 0),
				},
			},
		},
		{
			name: "version detects as linked",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path:    defaultPathDir,
					BaseURL: "https://example.com",
					Enabled: true,
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
				},
			},
			linkedRevisions: []Revision{NewRevision("16.3.0", 0)},
			inWindow:        true,

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  "https://example.com",
			linkedRevision:    NewRevision("16.3.0", 0),
			removedRevisions: []Revision{
				NewRevision("unknown-version", 0),
			},
			setupCalls: 1,
			restarted:  true,
		},
		{
			name: "backup version removed on install",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path:    defaultPathDir,
					BaseURL: "https://example.com",
					Enabled: true,
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
					Backup: toPtr(NewRevision("backup-version", 0)),
				},
			},
			inWindow: true,

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  "https://example.com",
			linkedRevision:    NewRevision("16.3.0", 0),
			removedRevisions: []Revision{
				NewRevision("backup-version", 0),
				NewRevision("unknown-version", 0),
			},
			setupCalls: 1,
			restarted:  true,
		},
		{
			name: "backup version is linked",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path:    defaultPathDir,
					BaseURL: "https://example.com",
					Enabled: true,
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
					Backup: toPtr(NewRevision("backup-version", 0)),
				},
			},
			inWindow:        true,
			linkedRevisions: []Revision{NewRevision("backup-version", 0)},

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  "https://example.com",
			linkedRevision:    NewRevision("16.3.0", 0),
			removedRevisions: []Revision{
				NewRevision("unknown-version", 0),
			},
			setupCalls: 1,
			restarted:  true,
		},
		{
			name: "backup version kept when no change",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path:    defaultPathDir,
					BaseURL: "https://example.com",
					Enabled: true,
				},
				Status: UpdateStatus{
					Active: NewRevision("16.3.0", 0),
					Backup: toPtr(NewRevision("backup-version", 0)),
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
					Path:    defaultPathDir,
					BaseURL: "https://example.com",
					Enabled: true,
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", autoupdate.FlagEnterprise|autoupdate.FlagFIPS),
					Backup: toPtr(NewRevision("backup-version", autoupdate.FlagEnterprise|autoupdate.FlagFIPS)),
				},
			},
			inWindow: true,
			flags:    autoupdate.FlagEnterprise | autoupdate.FlagFIPS,

			installedRevision: NewRevision("16.3.0", autoupdate.FlagEnterprise|autoupdate.FlagFIPS),
			installedBaseURL:  "https://example.com",
			linkedRevision:    NewRevision("16.3.0", autoupdate.FlagEnterprise|autoupdate.FlagFIPS),
			removedRevisions: []Revision{
				NewRevision("backup-version", autoupdate.FlagEnterprise|autoupdate.FlagFIPS),
				NewRevision("unknown-version", 0),
			},
			setupCalls: 1,
			restarted:  true,
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
					Path:    defaultPathDir,
					BaseURL: "https://example.com",
					Enabled: true,
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
					Backup: toPtr(NewRevision("backup-version", 0)),
				},
			},
			inWindow: true,
			setupErr: errors.New("setup error"),

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  "https://example.com",
			linkedRevision:    NewRevision("16.3.0", 0),
			removedRevisions: []Revision{
				NewRevision("backup-version", 0),
			},
			reloadCalls: 1,
			revertCalls: 1,
			setupCalls:  1,
			restarted:   true,
			errMatch:    "setup error",
		},
		{
			name: "agpl requires base URL",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path:    defaultPathDir,
					Enabled: true,
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
					Backup: toPtr(NewRevision("backup-version", 0)),
				},
			},
			inWindow: true,
			agpl:     true,

			reloadCalls: 0,
			revertCalls: 0,
			setupCalls:  0,
			errMatch:    "AGPL",
		},
		{
			name: "skip version",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path:    defaultPathDir,
					BaseURL: "https://example.com",
					Enabled: true,
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
					Backup: toPtr(NewRevision("backup-version", 0)),
					Skip:   toPtr(NewRevision("16.3.0", 0)),
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
					Path:    defaultPathDir,
					BaseURL: "https://example.com",
					Enabled: true,
					Pinned:  true,
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
					Backup: toPtr(NewRevision("backup-version", 0)),
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
				config.Edition = "community"
				if tt.flags&autoupdate.FlagEnterprise != 0 {
					config.Edition = "ent"
				}
				if tt.agpl {
					config.Edition = "oss"
				}
				config.FIPS = tt.flags&autoupdate.FlagFIPS != 0
				err := json.NewEncoder(w).Encode(config)
				require.NoError(t, err)
			}))
			t.Cleanup(server.Close)

			dir := t.TempDir()
			ns := &Namespace{
				installDir:     dir,
				defaultPathDir: "ignored",
			}
			_, err := ns.Init()
			require.NoError(t, err)
			cfgPath := filepath.Join(ns.Dir(), updateConfigName)
			ctx := context.Background()
			updater, err := NewLocalUpdater(ctx, LocalUpdaterConfig{
				InsecureSkipVerify: true,
			}, ns)
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
				installedRevision Revision
				installedBaseURL  string
				linkedRevision    Revision
				removedRevisions  []Revision
				revertFuncCalls   int
				setupCalls        int
				revertSetupCalls  int
				reloadCalls       int
			)
			updater.Installer = &testInstaller{
				FuncInstall: func(_ context.Context, rev Revision, baseURL string, force bool) error {
					for _, r := range tt.linkedRevisions {
						if r == rev {
							require.False(t, force)
						}
					}
					installedRevision = rev
					installedBaseURL = baseURL
					return tt.installErr
				},
				FuncLink: func(_ context.Context, rev Revision, path string, force bool) (revert func(context.Context) bool, err error) {
					linkedRevision = rev
					return func(_ context.Context) bool {
						revertFuncCalls++
						return true
					}, nil
				},
				FuncList: func(_ context.Context) (revs []Revision, err error) {
					return slices.Compact([]Revision{
						installedRevision,
						tt.cfg.Status.Active,
						NewRevision("unknown-version", 0),
					}), nil
				},
				FuncRemove: func(_ context.Context, rev Revision) error {
					removedRevisions = append(removedRevisions, rev)
					return nil
				},
				FuncIsLinked: func(ctx context.Context, rev Revision, path string) (bool, error) {
					for _, r := range tt.linkedRevisions {
						if r == rev {
							return true, nil
						}
					}
					return false, nil
				},
			}
			updater.Process = &testProcess{
				FuncReload: func(_ context.Context) error {
					reloadCalls++
					return tt.reloadErr
				},
				FuncIsPresent: func(ctx context.Context) (bool, error) {
					return true, nil
				},
				FuncIsEnabled: func(ctx context.Context) (bool, error) {
					return !tt.notEnabled, nil
				},
				FuncIsActive: func(ctx context.Context) (bool, error) {
					return !tt.notActive, nil
				},
			}
			var restarted bool
			updater.ReexecSetup = func(_ context.Context, path string, reload bool) error {
				restarted = reload
				setupCalls++
				return tt.setupErr
			}
			updater.SetupNamespace = func(_ context.Context, path string) error {
				revertSetupCalls++
				return nil
			}

			err = updater.Update(ctx, tt.now)
			if tt.errMatch != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.installedRevision, installedRevision)
			require.Equal(t, tt.installedBaseURL, installedBaseURL)
			require.Equal(t, tt.linkedRevision, linkedRevision)
			require.Equal(t, tt.removedRevisions, removedRevisions)
			require.Equal(t, tt.flags, installedRevision.Flags)
			require.Equal(t, tt.requestGroup, requestedGroup)
			require.Equal(t, tt.reloadCalls, reloadCalls)
			require.Equal(t, tt.revertCalls, revertSetupCalls)
			require.Equal(t, tt.revertCalls, revertFuncCalls)
			require.Equal(t, tt.setupCalls, setupCalls)
			require.Equal(t, tt.restarted, restarted)

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
		syncErr          error
		notPresent       bool

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
			name: "pinned",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Pinned: true,
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
		{
			name:               "systemd is not installed",
			tryLinkSystemCalls: 1,
			syncCalls:          1,
			syncErr:            ErrNotSupported,
		},
		{
			name: "systemd is not installed, already linked",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Enabled: false,
				},
			},
			tryLinkSystemCalls: 1,
			syncCalls:          1,
			syncErr:            ErrNotSupported,
		},
		{
			name: "SELinux blocks service from being read",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Enabled: false,
				},
			},
			tryLinkSystemCalls: 1,
			syncCalls:          1,
			notPresent:         true,
			errMatch:           "cannot find systemd service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			ns := &Namespace{installDir: dir}
			_, err := ns.Init()
			require.NoError(t, err)
			cfgPath := filepath.Join(ns.Dir(), updateConfigName)
			ctx := context.Background()
			updater, err := NewLocalUpdater(ctx, LocalUpdaterConfig{
				InsecureSkipVerify: true,
			}, ns)
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
					return tt.syncErr
				},
				FuncIsPresent: func(ctx context.Context) (bool, error) {
					return !tt.notPresent, nil
				},
			}

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

func TestUpdater_Remove(t *testing.T) {
	t.Parallel()

	const version = "active-version"

	tests := []struct {
		name           string
		cfg            *UpdateConfig // nil -> file not present
		linkSystemErr  error
		isEnabledErr   error
		syncErr        error
		reloadErr      error
		processEnabled bool
		force          bool

		unlinkedVersion string
		teardownCalls   int
		syncCalls       int
		revertFuncCalls int
		linkSystemCalls int
		reloadCalls     int
		errMatch        string
	}{
		{
			name:          "no config",
			teardownCalls: 1,
		},
		{
			name: "no active version",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
			},
			teardownCalls: 1,
		},
		{
			name: "no conflicting system links, process disabled, force",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Status: UpdateStatus{
					Active: NewRevision(version, 0),
				},
			},
			unlinkedVersion: version,
			teardownCalls:   1,
			force:           true,
		},
		{
			name: "no system links, process enabled, force",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path: defaultPathDir,
				},
				Status: UpdateStatus{
					Active: NewRevision(version, 0),
				},
			},
			linkSystemErr:   ErrNoBinaries,
			linkSystemCalls: 1,
			processEnabled:  true,
			force:           true,
			errMatch:        "refusing to remove",
		},
		{
			name: "no system links, process disabled, force",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path: defaultPathDir,
				},
				Status: UpdateStatus{
					Active: NewRevision(version, 0),
				},
			},
			linkSystemErr:   ErrNoBinaries,
			linkSystemCalls: 1,
			unlinkedVersion: version,
			teardownCalls:   1,
			force:           true,
		},
		{
			name: "no system links, process disabled, no force",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path: defaultPathDir,
				},
				Status: UpdateStatus{
					Active: NewRevision(version, 0),
				},
			},
			linkSystemErr:   ErrNoBinaries,
			linkSystemCalls: 1,
			errMatch:        "unable to remove",
		},
		{
			name: "no system links, process disabled, no systemd, force",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path: defaultPathDir,
				},
				Status: UpdateStatus{
					Active: NewRevision(version, 0),
				},
			},
			linkSystemErr:   ErrNoBinaries,
			linkSystemCalls: 1,
			isEnabledErr:    ErrNotSupported,
			unlinkedVersion: version,
			teardownCalls:   1,
			force:           true,
		},
		{
			name: "active version",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path: defaultPathDir,
				},
				Status: UpdateStatus{
					Active: NewRevision(version, 0),
				},
			},
			linkSystemCalls: 1,
			syncCalls:       1,
			reloadCalls:     1,
			teardownCalls:   1,
		},
		{
			name: "active version, no systemd",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path: defaultPathDir,
				},
				Status: UpdateStatus{
					Active: NewRevision(version, 0),
				},
			},
			linkSystemCalls: 1,
			syncCalls:       1,
			reloadCalls:     1,
			teardownCalls:   1,
			syncErr:         ErrNotSupported,
			reloadErr:       ErrNotSupported,
		},
		{
			name: "active version, no reload",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path: defaultPathDir,
				},
				Status: UpdateStatus{
					Active: NewRevision(version, 0),
				},
			},
			linkSystemCalls: 1,
			syncCalls:       1,
			reloadCalls:     1,
			teardownCalls:   1,
			reloadErr:       ErrNotNeeded,
		},
		{
			name: "active version, sync error",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path: defaultPathDir,
				},
				Status: UpdateStatus{
					Active: NewRevision(version, 0),
				},
			},
			linkSystemCalls: 1,
			syncCalls:       2,
			revertFuncCalls: 1,
			syncErr:         errors.New("sync error"),
			errMatch:        "configuration",
		},
		{
			name: "active version, reload error",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Path: defaultPathDir,
				},
				Status: UpdateStatus{
					Active: NewRevision(version, 0),
				},
			},
			linkSystemCalls: 1,
			syncCalls:       2,
			reloadCalls:     2,
			revertFuncCalls: 1,
			reloadErr:       errors.New("reload error"),
			errMatch:        "start",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			ns := &Namespace{installDir: dir}
			_, err := ns.Init()
			require.NoError(t, err)
			cfgPath := filepath.Join(ns.Dir(), updateConfigName)
			ctx := context.Background()
			updater, err := NewLocalUpdater(ctx, LocalUpdaterConfig{
				InsecureSkipVerify: true,
			}, ns)
			require.NoError(t, err)

			// Create config file only if provided in test case
			if tt.cfg != nil {
				b, err := yaml.Marshal(tt.cfg)
				require.NoError(t, err)
				err = os.WriteFile(cfgPath, b, 0600)
				require.NoError(t, err)
			}

			var (
				linkSystemCalls int
				revertFuncCalls int
				syncCalls       int
				reloadCalls     int
				teardownCalls   int
				unlinkedVersion string
			)
			updater.Installer = &testInstaller{
				FuncLinkSystem: func(_ context.Context) (revert func(context.Context) bool, err error) {
					linkSystemCalls++
					return func(_ context.Context) bool {
						revertFuncCalls++
						return true
					}, tt.linkSystemErr
				},
				FuncUnlink: func(_ context.Context, rev Revision, path string) error {
					unlinkedVersion = rev.Version
					return nil
				},
			}
			updater.Process = &testProcess{
				FuncSync: func(_ context.Context) error {
					syncCalls++
					return tt.syncErr
				},
				FuncReload: func(_ context.Context) error {
					reloadCalls++
					return tt.reloadErr
				},
				FuncIsEnabled: func(_ context.Context) (bool, error) {
					return tt.processEnabled, tt.isEnabledErr
				},
				FuncIsActive: func(_ context.Context) (bool, error) {
					return false, nil
				},
			}
			updater.TeardownNamespace = func(_ context.Context) error {
				teardownCalls++
				return nil
			}

			err = updater.Remove(ctx, tt.force)
			if tt.errMatch != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.syncCalls, syncCalls)
			require.Equal(t, tt.reloadCalls, reloadCalls)
			require.Equal(t, tt.linkSystemCalls, linkSystemCalls)
			require.Equal(t, tt.revertFuncCalls, revertFuncCalls)
			require.Equal(t, tt.unlinkedVersion, unlinkedVersion)
			require.Equal(t, tt.teardownCalls, teardownCalls)
		})
	}
}

func TestUpdater_Install(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		cfg        *UpdateConfig // nil -> file not present
		userCfg    OverrideConfig
		flags      autoupdate.InstallFlags
		agpl       bool
		installErr error
		setupErr   error
		reloadErr  error
		notPresent bool
		notEnabled bool
		notActive  bool

		removedRevision   Revision
		installedRevision Revision
		installedBaseURL  string
		linkedRevision    Revision
		requestGroup      string
		reloadCalls       int
		revertCalls       int
		setupCalls        int
		restarted         bool
		errMatch          string
	}{
		{
			name: "config from file",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Enabled: true,
					Group:   "group",
					Path:    "/path",
					BaseURL: "https://example.com",
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
				},
			},

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  "https://example.com",
			linkedRevision:    NewRevision("16.3.0", 0),
			requestGroup:      "group",
			setupCalls:        1,
			restarted:         true,
		},
		{
			name: "config from user",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					Group:   "old-group",
					BaseURL: "https://example.com/old",
				},
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
				},
			},
			userCfg: OverrideConfig{
				UpdateSpec: UpdateSpec{
					Enabled: true,
					Path:    "/path",
					Group:   "new-group",
					BaseURL: "https://example.com/new",
				},
				ForceVersion: "new-version",
			},

			installedRevision: NewRevision("new-version", 0),
			installedBaseURL:  "https://example.com/new",
			linkedRevision:    NewRevision("new-version", 0),
			requestGroup:      "new-group",
			setupCalls:        1,
			restarted:         true,
		},
		{
			name: "defaults",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
				},
			},

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  autoupdate.DefaultBaseURL,
			linkedRevision:    NewRevision("16.3.0", 0),
			setupCalls:        1,
			restarted:         true,
		},
		{
			name: "override skip",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
					Skip:   toPtr(NewRevision("16.3.0", 0)),
				},
			},

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  autoupdate.DefaultBaseURL,
			linkedRevision:    NewRevision("16.3.0", 0),
			setupCalls:        1,
			restarted:         true,
		},
		{
			name: "insecure URL",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Spec: UpdateSpec{
					BaseURL: "http://example.com",
				},
			},

			errMatch: "must use TLS",
		},
		{
			name: "install error",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
			},
			installErr: errors.New("install error"),

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  autoupdate.DefaultBaseURL,
			errMatch:          "install error",
		},
		{
			name: "agpl requires base URL",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
			},
			agpl:     true,
			errMatch: "AGPL",
		},
		{
			name: "version already installed",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Status: UpdateStatus{
					Active: NewRevision("16.3.0", 0),
				},
			},

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  autoupdate.DefaultBaseURL,
			linkedRevision:    NewRevision("16.3.0", 0),
			setupCalls:        1,
			restarted:         false,
		},
		{
			name: "backup version removed on install",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Status: UpdateStatus{
					Active: NewRevision("old-version", 0),
					Backup: toPtr(NewRevision("backup-version", 0)),
				},
			},

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  autoupdate.DefaultBaseURL,
			linkedRevision:    NewRevision("16.3.0", 0),
			removedRevision:   NewRevision("backup-version", 0),
			setupCalls:        1,
			restarted:         true,
		},
		{
			name: "backup version kept for validation",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Status: UpdateStatus{
					Active: NewRevision("16.3.0", 0),
					Backup: toPtr(NewRevision("backup-version", 0)),
				},
			},

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  autoupdate.DefaultBaseURL,
			linkedRevision:    NewRevision("16.3.0", 0),
			setupCalls:        1,
		},
		{
			name: "config does not exist",

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  autoupdate.DefaultBaseURL,
			linkedRevision:    NewRevision("16.3.0", 0),
			setupCalls:        1,
			restarted:         true,
		},
		{
			name:              "FIPS and Enterprise flags",
			flags:             autoupdate.FlagEnterprise | autoupdate.FlagFIPS,
			installedRevision: NewRevision("16.3.0", autoupdate.FlagEnterprise|autoupdate.FlagFIPS),
			installedBaseURL:  autoupdate.DefaultBaseURL,
			linkedRevision:    NewRevision("16.3.0", autoupdate.FlagEnterprise|autoupdate.FlagFIPS),
			setupCalls:        1,
			restarted:         true,
		},
		{
			name:     "invalid metadata",
			cfg:      &UpdateConfig{},
			errMatch: "invalid",
		},
		{
			name:     "setup fails",
			setupErr: errors.New("setup error"),

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  autoupdate.DefaultBaseURL,
			linkedRevision:    NewRevision("16.3.0", 0),
			revertCalls:       1,
			setupCalls:        1,
			reloadCalls:       1,
			restarted:         true,
			errMatch:          "setup error",
		},
		{
			name: "setup fails already installed",
			cfg: &UpdateConfig{
				Version: updateConfigVersion,
				Kind:    updateConfigKind,
				Status: UpdateStatus{
					Active: NewRevision("16.3.0", 0),
				},
			},
			setupErr: errors.New("setup error"),

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  autoupdate.DefaultBaseURL,
			linkedRevision:    NewRevision("16.3.0", 0),
			revertCalls:       1,
			setupCalls:        1,
			errMatch:          "setup error",
		},
		{
			name:      "no need to reload",
			reloadErr: ErrNotNeeded,

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  autoupdate.DefaultBaseURL,
			linkedRevision:    NewRevision("16.3.0", 0),
			setupCalls:        1,
			restarted:         true,
		},
		{
			name:       "not started or enabled",
			notEnabled: true,
			notActive:  true,

			installedRevision: NewRevision("16.3.0", 0),
			installedBaseURL:  autoupdate.DefaultBaseURL,
			linkedRevision:    NewRevision("16.3.0", 0),
			setupCalls:        1,
			restarted:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			ns := &Namespace{
				installDir:       dir,
				defaultPathDir:   defaultPathDir,
				defaultProxyAddr: "default-proxy",
			}
			_, err := ns.Init()
			require.NoError(t, err)
			cfgPath := filepath.Join(ns.Dir(), updateConfigName)
			ctx := context.Background()
			updater, err := NewLocalUpdater(ctx, LocalUpdaterConfig{
				InsecureSkipVerify: true,
			}, ns)
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
				config.Edition = "community"
				if tt.flags&autoupdate.FlagEnterprise != 0 {
					config.Edition = "ent"
				}
				if tt.agpl {
					config.Edition = "oss"
				}
				config.FIPS = tt.flags&autoupdate.FlagFIPS != 0
				err := json.NewEncoder(w).Encode(config)
				require.NoError(t, err)
			}))
			t.Cleanup(server.Close)

			if tt.userCfg.Proxy == "" {
				tt.userCfg.Proxy = strings.TrimPrefix(server.URL, "https://")
			}

			var (
				installedRevision Revision
				installedBaseURL  string
				linkedRevision    Revision
				removedRevision   Revision
				revertFuncCalls   int
				reloadCalls       int
				setupCalls        int
				revertSetupCalls  int
			)
			updater.Installer = &testInstaller{
				FuncInstall: func(_ context.Context, rev Revision, baseURL string, force bool) error {
					installedRevision = rev
					installedBaseURL = baseURL
					return tt.installErr
				},
				FuncLink: func(_ context.Context, rev Revision, path string, force bool) (revert func(context.Context) bool, err error) {
					linkedRevision = rev
					return func(_ context.Context) bool {
						revertFuncCalls++
						return true
					}, nil
				},
				FuncList: func(_ context.Context) (revs []Revision, err error) {
					return []Revision{}, nil
				},
				FuncRemove: func(_ context.Context, rev Revision) error {
					removedRevision = rev
					return nil
				},
				FuncIsLinked: func(ctx context.Context, rev Revision, path string) (bool, error) {
					return false, nil
				},
			}
			updater.Process = &testProcess{
				FuncReload: func(_ context.Context) error {
					reloadCalls++
					return tt.reloadErr
				},
				FuncIsPresent: func(ctx context.Context) (bool, error) {
					return !tt.notPresent, nil
				},
				FuncIsEnabled: func(ctx context.Context) (bool, error) {
					return !tt.notEnabled, nil
				},
				FuncIsActive: func(ctx context.Context) (bool, error) {
					return !tt.notActive, nil
				},
			}
			var restarted bool
			updater.ReexecSetup = func(_ context.Context, path string, reload bool) error {
				setupCalls++
				restarted = reload
				return tt.setupErr
			}
			updater.SetupNamespace = func(_ context.Context, path string) error {
				revertSetupCalls++
				return nil
			}

			err = updater.Install(ctx, tt.userCfg)
			if tt.errMatch != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.installedRevision, installedRevision)
			require.Equal(t, tt.installedBaseURL, installedBaseURL)
			require.Equal(t, tt.linkedRevision, linkedRevision)
			require.Equal(t, tt.removedRevision, removedRevision)
			require.Equal(t, tt.flags, installedRevision.Flags)
			require.Equal(t, tt.requestGroup, requestedGroup)
			require.Equal(t, tt.reloadCalls, reloadCalls)
			require.Equal(t, tt.revertCalls, revertSetupCalls)
			require.Equal(t, tt.revertCalls, revertFuncCalls)
			require.Equal(t, tt.setupCalls, setupCalls)
			require.Equal(t, tt.restarted, restarted)

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

func TestSameProxies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		a, b  string
		match bool
	}{
		{
			name:  "protocol missing with port",
			a:     "https://example.com:8080",
			b:     "example.com:8080",
			match: true,
		},
		{
			name:  "protocol missing without port",
			a:     "https://example.com",
			b:     "example.com",
			match: true,
		},
		{
			name:  "same with port",
			a:     "example.com:443",
			b:     "example.com:443",
			match: true,
		},
		{
			name:  "does not set default teleport port",
			a:     "example.com",
			b:     "example.com:3080",
			match: false,
		},
		{
			name:  "does set default standard port",
			a:     "example.com",
			b:     "example.com:443",
			match: true,
		},
		{
			name:  "other formats if equal",
			a:     "@123",
			b:     "@123",
			match: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := sameProxies(tt.a, tt.b)
			require.Equal(t, tt.match, s)
		})
	}
}

func TestUpdater_Setup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		restart    bool
		present    bool
		setupErr   error
		presentErr error
		reloadErr  error

		errMatch string
	}{
		{
			name:    "no restart",
			restart: false,
			present: true,
		},
		{
			name:    "restart",
			restart: true,
			present: true,
		},
		{
			name:      "reload not needed",
			restart:   true,
			present:   true,
			reloadErr: ErrNotNeeded,
		},
		{
			name:     "not present",
			restart:  true,
			present:  false,
			errMatch: "cannot find systemd",
		},
		{
			name:     "setup error",
			restart:  false,
			setupErr: errors.New("some error"),
			errMatch: "some error",
		},
		{
			name:     "setup error canceled",
			restart:  false,
			setupErr: context.Canceled,
			errMatch: "canceled",
		},
		{
			name:       "present error",
			restart:    false,
			presentErr: errors.New("some error"),
			errMatch:   "some error",
		},
		{
			name:       "present error canceled",
			restart:    false,
			presentErr: context.Canceled,
			errMatch:   "canceled",
		},
		{
			name:       "preset error not supported",
			restart:    false,
			presentErr: ErrNotSupported,
		},
		{
			name:      "reload error canceled",
			restart:   true,
			present:   true,
			reloadErr: context.Canceled,
			errMatch:  "canceled",
		},
		{
			name:      "reload error",
			restart:   true,
			present:   true,
			reloadErr: errors.New("some error"),
			errMatch:  "some error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := &Namespace{}
			ctx := context.Background()
			updater, err := NewLocalUpdater(ctx, LocalUpdaterConfig{}, ns)
			require.NoError(t, err)

			updater.Process = &testProcess{
				FuncReload: func(_ context.Context) error {
					return tt.reloadErr
				},
				FuncIsPresent: func(_ context.Context) (bool, error) {
					return tt.present, tt.presentErr
				},
			}
			updater.SetupNamespace = func(_ context.Context, path string) error {
				require.Equal(t, "test", path)
				return tt.setupErr
			}

			err = updater.Setup(ctx, "test", tt.restart)
			if tt.errMatch != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

var serverRegexp = regexp.MustCompile("127.0.0.1:[0-9]+")

func blankTestAddr(s []byte) []byte {
	return serverRegexp.ReplaceAll(s, []byte("localhost"))
}

type testInstaller struct {
	FuncInstall       func(ctx context.Context, rev Revision, baseURL string, force bool) error
	FuncRemove        func(ctx context.Context, rev Revision) error
	FuncLink          func(ctx context.Context, rev Revision, path string, force bool) (revert func(context.Context) bool, err error)
	FuncLinkSystem    func(ctx context.Context) (revert func(context.Context) bool, err error)
	FuncTryLink       func(ctx context.Context, rev Revision, path string) error
	FuncTryLinkSystem func(ctx context.Context) error
	FuncUnlink        func(ctx context.Context, rev Revision, path string) error
	FuncUnlinkSystem  func(ctx context.Context) error
	FuncList          func(ctx context.Context) (revs []Revision, err error)
	FuncIsLinked      func(ctx context.Context, rev Revision, path string) (bool, error)
}

func (ti *testInstaller) Install(ctx context.Context, rev Revision, baseURL string, force bool) error {
	return ti.FuncInstall(ctx, rev, baseURL, force)
}

func (ti *testInstaller) Remove(ctx context.Context, rev Revision) error {
	return ti.FuncRemove(ctx, rev)
}

func (ti *testInstaller) Link(ctx context.Context, rev Revision, path string, force bool) (revert func(context.Context) bool, err error) {
	return ti.FuncLink(ctx, rev, path, force)
}

func (ti *testInstaller) LinkSystem(ctx context.Context) (revert func(context.Context) bool, err error) {
	return ti.FuncLinkSystem(ctx)
}

func (ti *testInstaller) TryLink(ctx context.Context, rev Revision, path string) error {
	return ti.FuncTryLink(ctx, rev, path)
}

func (ti *testInstaller) TryLinkSystem(ctx context.Context) error {
	return ti.FuncTryLinkSystem(ctx)
}

func (ti *testInstaller) Unlink(ctx context.Context, rev Revision, path string) error {
	return ti.FuncUnlink(ctx, rev, path)
}

func (ti *testInstaller) UnlinkSystem(ctx context.Context) error {
	return ti.FuncUnlinkSystem(ctx)
}

func (ti *testInstaller) List(ctx context.Context) (revs []Revision, err error) {
	return ti.FuncList(ctx)
}

func (ti *testInstaller) IsLinked(ctx context.Context, rev Revision, path string) (bool, error) {
	return ti.FuncIsLinked(ctx, rev, path)
}

type testProcess struct {
	FuncReload    func(ctx context.Context) error
	FuncSync      func(ctx context.Context) error
	FuncIsEnabled func(ctx context.Context) (bool, error)
	FuncIsActive  func(ctx context.Context) (bool, error)
	FuncIsPresent func(ctx context.Context) (bool, error)
}

func (tp *testProcess) Reload(ctx context.Context) error {
	return tp.FuncReload(ctx)
}

func (tp *testProcess) Sync(ctx context.Context) error {
	return tp.FuncSync(ctx)
}

func (tp *testProcess) IsEnabled(ctx context.Context) (bool, error) {
	return tp.FuncIsEnabled(ctx)
}

func (tp *testProcess) IsActive(ctx context.Context) (bool, error) {
	return tp.FuncIsActive(ctx)
}

func (tp *testProcess) IsPresent(ctx context.Context) (bool, error) {
	return tp.FuncIsPresent(ctx)
}
