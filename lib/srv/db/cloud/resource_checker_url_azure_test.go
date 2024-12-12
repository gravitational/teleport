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

package cloud

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

func TestURLChecker_Azure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	testCases := types.Databases{
		mustMakeAzureDatabase(t, "mysql", defaults.ProtocolMySQL, "mysql.mysql.database.azure.com:3306", types.Azure{}),
		mustMakeAzureDatabase(t, "postgres", defaults.ProtocolPostgres, "postgres.postgres.database.azure.com:5432", types.Azure{}),
		mustMakeAzureDatabase(t, "redis", defaults.ProtocolRedis, "redis.redis.cache.windows.net:6380", types.Azure{
			ResourceID: "/subscriptions/<sub>/resourceGroups/<group>/providers/Microsoft.Cache/Redis/redis",
		}),
		mustMakeAzureDatabase(t, "redis-enterprise", defaults.ProtocolRedis, "redis-enterprise.region.redisenterprise.cache.azure.net", types.Azure{
			ResourceID: "/subscriptions/<sub>/resourceGroups/<group>/providers/Microsoft.Cache/redisEnterprise/databases/default",
		}),
		mustMakeAzureDatabase(t, "sqlserver", defaults.ProtocolSQLServer, "sqlserver.database.windows.net:1433", types.Azure{}),
	}

	c := newURLChecker(DiscoveryResourceCheckerConfig{
		Logger: utils.NewSlogLoggerForTests(),
	})
	for _, database := range testCases {
		t.Run(database.GetName(), func(t *testing.T) {
			t.Run("valid", func(t *testing.T) {
				require.NoError(t, c.Check(ctx, database))
			})

			// Make a copy and set an invalid URI.
			t.Run("invalid", func(t *testing.T) {
				invalid := database.Copy()
				invalid.SetURI("localhost:12345")
				require.Error(t, c.Check(ctx, invalid))
			})
		})
	}
}

func mustMakeAzureDatabase(t *testing.T, name, protocol, uri string, azure types.Azure) types.Database {
	t.Helper()

	database, err := types.NewDatabaseV3(
		types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			URI:      uri,
			Protocol: protocol,
			Azure:    azure,
		},
	)
	require.NoError(t, err)
	return database
}
