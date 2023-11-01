/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
		AdminUser: types.DatabaseAdminUser{
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
