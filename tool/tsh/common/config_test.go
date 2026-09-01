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

package common

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/config/openssh"
)

// TestWriteSSHConfig tests the writeSSHConfig template output.
func TestWriteSSHConfig(t *testing.T) {
	t.Parallel()

	want := `# Begin generated Teleport configuration for localhost by tsh

# Common flags for all test-cluster hosts
Host *.test-cluster localhost
    UserKnownHostsFile "/tmp/know_host"
    IdentityFile "/tmp/alice"
    CertificateFile "/tmp/localhost-cert.pub"

# Flags for all test-cluster hosts except the proxy
Host *.test-cluster !localhost
    Port 3022
    ProxyCommand "/bin/tsh" proxy ssh --cluster=test-cluster --proxy=localhost:3080 %r@%h:%p

# End generated Teleport configuration
`

	var sb strings.Builder
	err := writeSSHConfig(&sb, &openssh.SSHConfigParameters{
		AppName:             "tsh",
		ClusterNames:        []string{"test-cluster"},
		KnownHostsPath:      "/tmp/know_host",
		IdentityFilePath:    "/tmp/alice",
		CertificateFilePath: "/tmp/localhost-cert.pub",
		ProxyHost:           "localhost",
		ProxyPort:           "3080",
		ExecutablePath:      "/bin/tsh",
	})
	require.NoError(t, err)
	require.Equal(t, want, sb.String())
}

func TestIsNonInteractiveWriter(t *testing.T) {
	t.Parallel()

	t.Run("pipe", func(t *testing.T) {
		reader, writer, err := os.Pipe()
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, writer.Close())
			require.NoError(t, reader.Close())
		})

		require.True(t, isNonInteractiveWriter(writer))
	})

	t.Run("redirected to file", func(t *testing.T) {
		file, err := os.CreateTemp(t.TempDir(), "stdout-*")
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, file.Close())
		})

		require.True(t, isNonInteractiveWriter(file))
	})

	t.Run("character device", func(t *testing.T) {
		file, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, file.Close())
		})

		require.False(t, isNonInteractiveWriter(file))
	})
}
