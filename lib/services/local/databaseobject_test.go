/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobject"
)

// TestDatabaseObjectCRUD tests backend operations with DatabaseObject resources.
func TestDatabaseObjectCRUD(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service, err := NewDatabaseObjectService(backend)
	require.NoError(t, err)

	// Create a couple objects.
	obj1, err := databaseobject.NewDatabaseObject("obj1", &dbobjectv1.DatabaseObjectSpec{Name: "obj1", Protocol: "postgres"})
	require.NoError(t, err)

	obj2, err := databaseobject.NewDatabaseObject("obj2", &dbobjectv1.DatabaseObjectSpec{Name: "obj2", Protocol: "postgres"})
	require.NoError(t, err)

	// Initially we expect no objects.
	out, nextToken, err := service.ListDatabaseObjects(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)

	// Create both objects.
	obj, err := service.CreateDatabaseObject(ctx, obj1)
	require.NoError(t, err)
	require.Equal(t, obj1, obj)

	obj, err = service.CreateDatabaseObject(ctx, obj2)
	require.NoError(t, err)
	require.Equal(t, obj2, obj)

	// Fetch all objects.
	out, nextToken, err = service.ListDatabaseObjects(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Len(t, out, 2)
	require.Equal(t, obj1.String(), out[0].String())
	require.Equal(t, obj2.String(), out[1].String())

	// Fetch a paginated list of objects
	paginatedOut := make([]*dbobjectv1.DatabaseObject, 0, 2)
	for {
		out, nextToken, err = service.ListDatabaseObjects(ctx, 1, nextToken)
		require.NoError(t, err)

		paginatedOut = append(paginatedOut, out...)
		if nextToken == "" {
			break
		}
	}

	require.True(t, proto.Equal(obj1, paginatedOut[0]))
	require.True(t, proto.Equal(obj2, paginatedOut[1]))

	// Fetch a specific object.
	obj, err = service.GetDatabaseObject(ctx, obj2.Metadata.GetName())
	require.NoError(t, err)
	require.True(t, proto.Equal(obj, obj2))

	// Try to fetch an object that doesn't exist.
	_, err = service.GetDatabaseObject(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

	// Try to create the same object.
	_, err = service.CreateDatabaseObject(ctx, obj1)
	require.True(t, trace.IsAlreadyExists(err), "expected already exists error, got %v", err)

	// Update an object.
	obj1.Metadata.Expires = timestamppb.New(clock.Now().Add(30 * time.Minute))
	_, err = service.UpdateDatabaseObject(ctx, obj1)
	require.NoError(t, err)
	obj, err = service.GetDatabaseObject(ctx, obj1.GetMetadata().GetName())
	require.NoError(t, err)
	//nolint:staticcheck // SA1019. Deprecated, but still needed.
	obj.Metadata.Id = obj1.Metadata.Id
	require.True(t, proto.Equal(obj, obj1))

	// Delete an object
	err = service.DeleteDatabaseObject(ctx, obj1.GetMetadata().GetName())
	require.NoError(t, err)
	out, nextToken, err = service.ListDatabaseObjects(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.True(t, proto.Equal(obj2, out[0]))

	// Try to delete an object that doesn't exist.
	err = service.DeleteDatabaseObject(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err), "expected not found error, got %v", err)

	// Delete all resources.
	lst, nextToken, err := service.ListDatabaseObjects(ctx, 200, "")
	require.NoError(t, err)
	require.Equal(t, "", nextToken)
	for _, elem := range lst {
		err = service.DeleteDatabaseObject(ctx, elem.GetMetadata().GetName())
		require.NoError(t, err)
	}
	out, nextToken, err = service.ListDatabaseObjects(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)
}

func TestMarshalDatabaseObjectRoundTrip(t *testing.T) {
	spec := &dbobjectv1.DatabaseObjectSpec{Name: "dummy", Protocol: "postgres"}
	obj, err := databaseobject.NewDatabaseObject("dummy-table", spec)
	require.NoError(t, err)

	out, err := marshalDatabaseObject(obj)
	require.NoError(t, err)
	newObj, err := unmarshalDatabaseObject(out)
	require.NoError(t, err)
	require.True(t, proto.Equal(obj, newObj), "messages are not equal")
}
