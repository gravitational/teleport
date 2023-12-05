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

package role

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

func TestGetDatabaseRoleMatchers(t *testing.T) {
	postgresDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name: "postgres",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		AdminUser: &types.DatabaseAdminUser{
			Name: "teleport-admin",
		},
	})
	require.NoError(t, err)

	mysqlDatabase, err := types.NewDatabaseV3(types.Metadata{
		Name: "mysql",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost:3306",
	})
	require.NoError(t, err)

	require.NoError(t, err)
	tests := []struct {
		name               string
		inputConfig        RoleMatchersConfig
		expectRoleMatchers services.RoleMatchers
	}{
		{
			name: "database name matcher required",
			inputConfig: RoleMatchersConfig{
				Database:     postgresDatabase,
				DatabaseUser: "alice",
				DatabaseName: "db1",
			},
			expectRoleMatchers: services.RoleMatchers{
				services.NewDatabaseUserMatcher(postgresDatabase, "alice"),
				&services.DatabaseNameMatcher{Name: "db1"},
			},
		},
		{
			name: "database name matcher not required",
			inputConfig: RoleMatchersConfig{
				Database:     mysqlDatabase,
				DatabaseUser: "alice",
				DatabaseName: "db1",
			},
			expectRoleMatchers: services.RoleMatchers{
				services.NewDatabaseUserMatcher(postgresDatabase, "alice"),
			},
		},
		{
			name: "AutoCreateUser",
			inputConfig: RoleMatchersConfig{
				Database:       postgresDatabase,
				DatabaseUser:   "alice",
				DatabaseName:   "db1",
				AutoCreateUser: true,
			},
			expectRoleMatchers: services.RoleMatchers{
				&services.DatabaseNameMatcher{Name: "db1"},
			},
		},
		{
			name: "DisableDatabaseNameMatcher",
			inputConfig: RoleMatchersConfig{
				Database:                   postgresDatabase,
				DatabaseUser:               "alice",
				DatabaseName:               "db1",
				DisableDatabaseNameMatcher: true,
			},
			expectRoleMatchers: services.RoleMatchers{
				services.NewDatabaseUserMatcher(postgresDatabase, "alice"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.EqualValues(t, test.expectRoleMatchers, GetDatabaseRoleMatchers(test.inputConfig))
		})
	}
}
