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
	"bytes"
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestInstallSystemdCmd(t *testing.T) {
	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()

	// Create pre-existing file to test --force
	preExistingDir := t.TempDir()
	err := os.WriteFile(filepath.Join(preExistingDir, "tbot.service"), []byte("pre-existing"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name           string
		params         []string
		usePreExisting bool

		wantUnitName string
		wantErr      string
		wantStdout   bool
	}{
		{
			name: "success - defaults",
			params: []string{
				"--write",
			},
		},
		{
			name:       "success - defaults and dry run",
			params:     []string{},
			wantStdout: true,
		},
		{
			name: "success - overrides",
			params: []string{
				"--write",
				"--name", "my-farm-bot",
				"--group", "llamas",
				"--user", "llama",
				"--anonymous-telemetry",
			},
			wantUnitName: "my-farm-bot",
		},
		{
			name: "fails prexisting",
			params: []string{
				"--write",
				"--group", "llamas",
				"--user", "llama",
			},
			wantErr:        "already exists with different content",
			usePreExisting: true,
		},
		{
			name: "succeeds prexisting with force",
			params: []string{
				"--write",
				"--group", "llamas",
				"--user", "llama",
				"--force",
			},
			usePreExisting: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dirPath string
			if tt.usePreExisting {
				dirPath = preExistingDir
			} else {
				dirPath = t.TempDir()
			}
			params := append(
				tt.params, []string{"--systemd-directory", dirPath}...,
			)

			app := kingpin.New("test", "")
			installSystemdCmdStr, installSystemdCmd := setupInstallSystemdCmd(app)
			cmd, err := app.Parse(append([]string{"install", "systemd"}, params...))
			require.NoError(t, err)
			require.Equal(t, installSystemdCmdStr, cmd)

			stdout := bytes.NewBuffer(nil)

			err = installSystemdCmd(ctx, log, "/etc/tbot.yaml", func() (string, error) {
				return "/usr/local/bin/tbot", nil
			}, stdout)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			var data []byte
			if tt.wantStdout {
				// Ensure that in dry run, no actual output is written!
				_, err = os.ReadFile(
					filepath.Join(dirPath, fmt.Sprintf("%s.service", cmp.Or(tt.wantUnitName, "tbot"))),
				)
				require.ErrorIs(t, err, os.ErrNotExist)
				data = stdout.Bytes()
				data = bytes.ReplaceAll(data, []byte(dirPath), []byte("/test/dir"))
			} else {
				data, err = os.ReadFile(
					filepath.Join(dirPath, fmt.Sprintf("%s.service", cmp.Or(tt.wantUnitName, "tbot"))),
				)
				require.NoError(t, err)
			}

			if golden.ShouldSet() {
				golden.Set(t, data)
			}
			require.Equal(t, string(golden.Get(t)), string(data))
		})
	}
}
