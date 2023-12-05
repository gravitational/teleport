/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package config

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils/golden"
)

func TestTemplateSSHClient_Render(t *testing.T) {
	tests := []struct {
		Name    string
		Version string
	}{
		{
			Name:    "legacy OpenSSH",
			Version: "6.5.0",
		},
		{
			Name:    "latest OpenSSH",
			Version: "9.0.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			dir := t.TempDir()

			cfg, err := newTestConfig("example.com")
			require.NoError(t, err)

			// identity is passed in, but not used.
			var ident *identity.Identity
			dest := &DestinationDirectory{
				Path:     dir,
				Symlinks: botfs.SymlinksInsecure,
				ACLs:     botfs.ACLOff,
			}

			mockBot := newMockProvider(cfg)
			tmpl := templateSSHClient{
				getSSHVersion: func() (*semver.Version, error) {
					return semver.New(tc.Version), nil
				},
				executablePathGetter: fakeGetExecutablePath,
				destPath:             dest.Path,
			}

			err = tmpl.render(context.Background(), mockBot, ident, dest)
			require.NoError(t, err)

			replaceTestDir := func(b []byte) []byte {
				return bytes.ReplaceAll(b, []byte(dir), []byte("/test/dir"))
			}

			knownHostBytes, err := os.ReadFile(filepath.Join(dir, knownHostsName))
			require.NoError(t, err)
			knownHostBytes = replaceTestDir(knownHostBytes)
			sshConfigBytes, err := os.ReadFile(filepath.Join(dir, sshConfigName))
			require.NoError(t, err)
			sshConfigBytes = replaceTestDir(sshConfigBytes)
			if golden.ShouldSet() {
				golden.SetNamed(t, "known_hosts", knownHostBytes)
				golden.SetNamed(t, "ssh_config", sshConfigBytes)
			}
			require.Equal(
				t, string(golden.GetNamed(t, "known_hosts")), string(knownHostBytes),
			)
			require.Equal(
				t, string(golden.GetNamed(t, "ssh_config")), string(sshConfigBytes),
			)
		})
	}
}
