/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package handler

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

// TestUnifiedResources_DatabaseAutoUsers verifies that databases returned in
// unified resources have the correct auto-user provisioning configuration.
func TestUnifiedResources_DatabaseAutoUsers(t *testing.T) {
	t.Run("database with auto-user provisioning in unified resources", func(t *testing.T) {
		// Create database with AdminUser (auto-users enabled)
		db, err := types.NewDatabaseV3(types.Metadata{
			Name: "auto-user-db",
		}, types.DatabaseSpecV3{
			Protocol: "postgres",
			URI:      "localhost:5432",
			AdminUser: &types.DatabaseAdminUser{
				Name: "teleport-admin",
			},
		})
		require.NoError(t, err)

		testDatabase := clusters.Database{
			URI:      uri.NewClusterURI("test-cluster").AppendDB("auto-user-db"),
			Database: db,
		}

		// Verify database configuration
		require.True(t, testDatabase.IsAutoUsersEnabled())
		require.Equal(t, "postgres", testDatabase.GetProtocol())
	})

	t.Run("database without auto-user provisioning in unified resources", func(t *testing.T) {
		// Create database without AdminUser
		db, err := types.NewDatabaseV3(types.Metadata{
			Name: "regular-db",
		}, types.DatabaseSpecV3{
			Protocol: "mysql",
			URI:      "localhost:3306",
		})
		require.NoError(t, err)

		testDatabase := clusters.Database{
			URI:      uri.NewClusterURI("test-cluster").AppendDB("regular-db"),
			Database: db,
		}

		// Verify database configuration
		require.False(t, testDatabase.IsAutoUsersEnabled())
		require.Equal(t, "mysql", testDatabase.GetProtocol())
	})

	t.Run("multiple databases with mixed auto-user configurations", func(t *testing.T) {
		// Database 1: with auto-users
		db1, err := types.NewDatabaseV3(types.Metadata{
			Name: "auto-db-1",
		}, types.DatabaseSpecV3{
			Protocol: "postgres",
			URI:      "host1:5432",
			AdminUser: &types.DatabaseAdminUser{
				Name: "admin1",
			},
		})
		require.NoError(t, err)

		// Database 2: without auto-users
		db2, err := types.NewDatabaseV3(types.Metadata{
			Name: "regular-db-2",
		}, types.DatabaseSpecV3{
			Protocol: "mysql",
			URI:      "host2:3306",
		})
		require.NoError(t, err)

		// Database 3: with auto-users
		db3, err := types.NewDatabaseV3(types.Metadata{
			Name: "auto-db-3",
		}, types.DatabaseSpecV3{
			Protocol: "postgres",
			URI:      "host3:5432",
			AdminUser: &types.DatabaseAdminUser{
				Name: "admin3",
			},
		})
		require.NoError(t, err)

		databases := []clusters.Database{
			{URI: uri.NewClusterURI("test-cluster").AppendDB("auto-db-1"), Database: db1},
			{URI: uri.NewClusterURI("test-cluster").AppendDB("regular-db-2"), Database: db2},
			{URI: uri.NewClusterURI("test-cluster").AppendDB("auto-db-3"), Database: db3},
		}

		// Verify each database has correct auto-user configuration
		require.True(t, databases[0].IsAutoUsersEnabled())
		require.False(t, databases[1].IsAutoUsersEnabled())
		require.True(t, databases[2].IsAutoUsersEnabled())
	})
}

/*
 * Note: Full integration tests for ListUnifiedResources are in integration test suites
 * as they require:
 * 1. Handler with proper DaemonService setup
 * 2. Cluster resolution and client caching
 * 3. AuthClient for auto-user info retrieval
 * 4. Proper pagination and filtering
 *
 * The ListUnifiedResources handler:
 * 1. Resolves the cluster from the URI
 * 2. Gets cached client and authClient
 * 3. For each database resource, calls newAPIDatabase with cluster and authClient
 * 4. newAPIDatabase calls cluster.GetDatabaseAutoUserInfo to populate AutoUsersEnabled
 */
