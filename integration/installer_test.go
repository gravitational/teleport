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

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/automaticupgrades/installer"
)

func TestInstallTeleport(t *testing.T) {

	testCases := []struct {
		desc string
		arch string
	}{
		{
			desc: "Install Linux 32-bit",
			arch: "386",
		},
		{
			desc: "Install Linux 64-bit",
			arch: "amd64",
		},
		{
			desc: "Install Linux ARM64",
			arch: "arm64",
		},
		{
			desc: "Install Linux ARMv7",
			arch: "arm",
		},
	}

	testDir := filepath.Join(os.TempDir(), "teleport-unit-test", "bin")
	defer os.RemoveAll(testDir)

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			binDir := filepath.Join(testDir, tc.arch)
			require.NoError(t, os.MkdirAll(binDir, os.ModePerm))
			defer os.RemoveAll(binDir)

			teleportInstaller, err := installer.NewTeleportInstaller(installer.Config{
				TeleportBinDir: binDir,
			})
			require.NoError(t, err)

			err = teleportInstaller.InstallTeleport(context.Background(), installer.Request{
				Version: "v15.1.10",
				Arch:    tc.arch,
				OS:      "linux",
				Flavor:  "teleport-ent",
			})
			require.NoError(t, err)
		})
	}
}
