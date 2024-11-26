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

package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSSHProxyCommand(t *testing.T) {
	testCommand(t, NewSSHProxyCommand, []testCommandCase[*SSHProxyCommand]{
		{
			name: "success",
			args: []string{
				"ssh-proxy-command",
				"--destination-dir=/bar",
				"--cluster=foo",
				"--user=noah",
				"--host=example.com",
				"--proxy-server=example.com:443",
				"--tls-routing",
				"--connection-upgrade",
				"--proxy-templates=/tmp/tsh.yaml",
				"--resume",
			},
			assert: func(t *testing.T, got *SSHProxyCommand) {
				require.Equal(t, "/bar", got.DestinationDir)
				require.Equal(t, "foo", got.Cluster)
				require.Equal(t, "noah", got.User)
				require.Equal(t, "example.com", got.Host)
				require.Equal(t, "example.com:443", got.ProxyServer)
				require.True(t, got.TLSRoutingEnabled)
				require.True(t, got.ConnectionUpgradeRequired)
				require.Equal(t, "/tmp/tsh.yaml", got.TSHConfigPath)
				require.True(t, got.EnableResumption)
			},
		},
	})
}
