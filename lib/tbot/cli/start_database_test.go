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

	"github.com/gravitational/teleport/lib/tbot/config"
)

// TestDatabaseCommand tests that the DatabaseCommand properly parses its
// arguments and applies as expected onto a BotConfig.
func TestDatabaseCommand(t *testing.T) {
	testStartConfigureCommand(t, NewDatabaseCommand, []startConfigureTestCase{
		{
			name: "success",
			args: []string{
				"start",
				"database",
				"--destination=/bar",
				"--token=foo",
				"--join-method=github",
				"--proxy-server=example.com:443",
				"--service=foo",
				"--username=bar",
				"--database=baz",
				"--format=tls",
			},
			assertConfig: func(t *testing.T, cfg *config.BotConfig) {
				require.Len(t, cfg.Services, 1)

				// It must configure a database output with a directory destination.
				svc := cfg.Services[0]
				db, ok := svc.(*config.DatabaseOutput)
				require.True(t, ok)

				require.Equal(t, "foo", db.Service)
				require.Equal(t, "bar", db.Username)
				require.Equal(t, "baz", db.Database)
				require.Equal(t, config.TLSDatabaseFormat, db.Format)

				dir, ok := db.Destination.(*config.DestinationDirectory)
				require.True(t, ok)
				require.Equal(t, "/bar", dir.Path)
			},
		},
	})
}
