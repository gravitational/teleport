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

func TestNewAPIDatabase_Fields(t *testing.T) {
	t.Run("populates basic database fields", func(t *testing.T) {
		db, err := types.NewDatabaseV3(types.Metadata{
			Name:        "test-db",
			Description: "Test database",
			Labels: map[string]string{
				"env":  "test",
				"tier": "backend",
			},
		}, types.DatabaseSpecV3{
			Protocol: "postgres",
			URI:      "localhost:5432",
		})
		require.NoError(t, err)

		testDatabase := clusters.Database{
			URI:      uri.NewClusterURI("test-cluster").AppendDB("test-db"),
			Database: db,
		}

		require.Equal(t, "test-db", testDatabase.GetName())
		require.Equal(t, "Test database", testDatabase.GetDescription())
		require.Equal(t, "postgres", testDatabase.GetProtocol())
		require.Equal(t, "localhost:5432", testDatabase.GetURI())
	})

	t.Run("database with auto-user provisioning enabled", func(t *testing.T) {
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

		require.True(t, testDatabase.IsAutoUsersEnabled())
		require.NotNil(t, testDatabase.GetAdminUser())
		require.Equal(t, "teleport-admin", testDatabase.GetAdminUser().Name)
	})
}
