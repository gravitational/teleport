// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
