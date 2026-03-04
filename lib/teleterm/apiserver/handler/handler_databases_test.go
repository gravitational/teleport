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
			URI:                uri.NewClusterURI("test-cluster").AppendDB("test-db"),
			Database:           db,
			AutoUsersEnabled:   true,
			DatabaseRoles:      []string{"reader", "writer"},
			AutoUserDbUsername: "alice",
		}

		apiDB := newAPIDatabase(testDatabase)

		require.Equal(t, "/clusters/test-cluster/dbs/test-db", apiDB.Uri)
		require.Equal(t, "test-db", apiDB.Name)
		require.Equal(t, "Test database", apiDB.Desc)
		require.Equal(t, "postgres", apiDB.Protocol)
		require.NotNil(t, apiDB.AutoUserProvisioning)
		require.Equal(t, []string{"reader", "writer"}, apiDB.AutoUserProvisioning.DatabaseRoles)
		require.Equal(t, "alice", apiDB.AutoUserProvisioning.Username)
		require.NotNil(t, apiDB.Labels)
	})
}
