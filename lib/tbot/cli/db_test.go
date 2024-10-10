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

func TestDBCommand(t *testing.T) {
	testCommand(t, NewDBCommand, []testCommandCase[*DBCommand]{
		{
			name: "success",
			args: []string{
				"db",
				"--proxy-server=example.com:22",
				"--destination-dir=/tmp",
				"--cluster=example.com",
				"bar",
				"buzz",
				"boo",
			},
			assert: func(t *testing.T, got *DBCommand) {
				require.Equal(t, "example.com:22", got.ProxyServer)
				require.Equal(t, "/tmp", got.DestinationDir)
				require.Equal(t, "example.com", got.Cluster)
				require.Equal(t, []string{"bar", "buzz", "boo"}, *got.RemainingArgs)
			},
		},
	})
}
