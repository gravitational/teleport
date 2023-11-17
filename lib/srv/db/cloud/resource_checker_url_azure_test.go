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

package cloud

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

func TestURLChecker_Azure(t *testing.T) {
	t.Parallel()

	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
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
		Log: log,
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
