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

package local

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
)

// TestDatabasesCRUD tests backend operations with database resources.
func TestDatabasesCRUD(t *testing.T) {
	ctx := context.Background()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service := NewDatabasesService(backend)

	// Create a couple databases.
	db1, err := types.NewDatabaseV3(types.Metadata{
		Name: "db1",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)
	db2, err := types.NewDatabaseV3(types.Metadata{
		Name: "db2",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost:3306",
	})
	require.NoError(t, err)

	// Initially we expect no databases.
	out, err := service.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, out)

	// Create both databases.
	err = service.CreateDatabase(ctx, db1)
	require.NoError(t, err)
	err = service.CreateDatabase(ctx, db2)
	require.NoError(t, err)

	// Try to create an invalid database.
	dbBadURI, err := types.NewDatabaseV3(types.Metadata{
		Name: "db-missing-port",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost",
	})
	require.NoError(t, err)
	require.NoError(t, service.CreateDatabase(ctx, dbBadURI))

	// Fetch all databases.
	out, err = service.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{dbBadURI, db1, db2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Fetch a specific database.
	db, err := service.GetDatabase(ctx, db2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(db2, db,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Try to fetch a database that doesn't exist.
	_, err = service.GetDatabase(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Try to create the same database.
	err = service.CreateDatabase(ctx, db1)
	require.IsType(t, trace.AlreadyExists(""), err)

	// Update a database.
	db1.Metadata.Description = "description"
	err = service.UpdateDatabase(ctx, db1)
	require.NoError(t, err)
	db, err = service.GetDatabase(ctx, db1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(db1, db,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Delete a database.
	err = service.DeleteDatabase(ctx, db1.GetName())
	require.NoError(t, err)
	out, err = service.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{dbBadURI, db2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Try to delete a database that doesn't exist.
	err = service.DeleteDatabase(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Delete all databases.
	err = service.DeleteAllDatabases(ctx)
	require.NoError(t, err)
	out, err = service.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, out)
}
