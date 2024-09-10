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

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/utils/golden"
)

func TestRun_Disable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		cfg      *UpdateConfig // nil -> file not present
		errMatch string
	}{
		{
			name: "enabled",
			args: []string{"disable"},
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
			args: []string{"disable"},
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
			args: []string{"disable"},
		},
		{
			name: "invalid metadata",
			args: []string{"disable"},
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
			path := filepath.Join(dir, versionsDirName, configFileName)

			// Create config file only if provided in test case
			if tt.cfg != nil {
				err := os.MkdirAll(filepath.Dir(path), 0777)
				require.NoError(t, err)
				b, err := yaml.Marshal(tt.cfg)
				require.NoError(t, err)
				err = os.WriteFile(path, b, 0600)
				require.NoError(t, err)
			}

			args := append(tt.args, []string{"--data-dir", dir}...)
			err := Run(args)

			if tt.errMatch != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
				return
			}
			require.NoError(t, err)

			data, err := os.ReadFile(path)

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

func TestRun_Lock(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lockfile := filepath.Join(dir, lockFileName)
	unlock, err := lock(lockfile, true)
	require.NoError(t, err)

	_, err = lock(lockfile, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")

	err = unlock()
	require.NoError(t, err)

	unlock2, err := lock(lockfile, true)
	require.NoError(t, err)
	err = unlock2()
	require.NoError(t, err)
}
