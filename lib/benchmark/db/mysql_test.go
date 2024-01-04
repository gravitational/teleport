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

package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMySQLConnString(t *testing.T) {
	for name, tc := range map[string]struct {
		connString string
		expectErr  bool
		host       string
		username   string
		password   string
		database   string
	}{
		"valid": {
			connString: "mysql://user_name:123123@localhost:3333/db?get-server-public-key=true",
			host:       "localhost:3333",
			username:   "user_name",
			password:   "123123",
			database:   "db",
		},
		"valid without database": {
			connString: "mysql://user@localhost:3306?get-server-public-key=true",
			host:       "localhost:3306",
			username:   "user",
		},
		"valid with escaped char": {
			connString: "mysql://user_name@198.51.100.2:33060/world%5Fx",
			host:       "198.51.100.2:33060",
			username:   "user_name",
			database:   "world_x",
		},
		"unsupported scheme": {
			connString: "mysqlx://user_name@localhost:33065",
			expectErr:  true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			config, err := parseMySQLConnString(tc.connString)
			if tc.expectErr {
				require.Error(t, err)
				return
			}

			require.Equal(t, tc.host, config.host)
			require.Equal(t, tc.username, config.username)
			require.Equal(t, tc.password, config.password)
			require.Equal(t, tc.database, config.database)
		})
	}
}
