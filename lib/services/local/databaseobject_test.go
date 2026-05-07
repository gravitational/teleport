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
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobject"
)

func getService(t *testing.T) services.DatabaseObjects {
	backend, err := memory.New(memory.Config{
		Context: context.Background(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service, err := NewDatabaseObjectService(backend)
	require.NoError(t, err)
	return service
}

func getObject(t *testing.T, index int) *dbobjectv1.DatabaseObject {
	name := fmt.Sprintf("obj%v", index)
	obj, err := databaseobject.NewDatabaseObject(name, &dbobjectv1.DatabaseObjectSpec{Name: name, Protocol: "postgres"})
	require.NoError(t, err)

	return obj
}

func prepopulate(t *testing.T, service services.DatabaseObjects, count int) {
	for i := 0; i < count; i++ {
		_, err := service.CreateDatabaseObject(context.Background(), getObject(t, i))
		require.NoError(t, err)
	}
}

func TestCreateDatabaseObjects(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getService(t)

	obj, err := databaseobject.NewDatabaseObject("obj", &dbobjectv1.DatabaseObjectSpec{Name: "obj", Protocol: "postgres"})
	require.NoError(t, err)

	// first attempt should succeed
	objOut, err := service.CreateDatabaseObject(ctx, obj)
	require.NoError(t, err)
	require.Equal(t, obj, objOut)

	// second attempt should fail, object already exists
	_, err = service.CreateDatabaseObject(ctx, obj)
	require.Error(t, err)
}

func TestUpsertDatabaseObjects(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getService(t)

	obj, err := databaseobject.NewDatabaseObject("obj", &dbobjectv1.DatabaseObjectSpec{Name: "obj", Protocol: "postgres"})
	require.NoError(t, err)

	// first attempt should succeed
	objOut, err := service.UpsertDatabaseObject(ctx, obj)
	require.NoError(t, err)
	require.Equal(t, obj, objOut)

	// second attempt should also succeed
	objOut, err = service.UpsertDatabaseObject(ctx, obj)
	require.NoError(t, err)
	require.Equal(t, obj, objOut)
}

func TestGetDatabaseObject(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getService(t)
	prepopulate(t, service, 1)

	tests := []struct {
		name    string
		key     string
		wantErr bool
		wantObj *dbobjectv1.DatabaseObject
	}{
		{
			name:    "object does not exist",
			key:     "dummy",
			wantErr: true,
			wantObj: nil,
		},
		{
			name:    "success",
			key:     getObject(t, 0).GetMetadata().GetName(),
			wantErr: false,
			wantObj: getObject(t, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fetch a specific object.
			obj, err := service.GetDatabaseObject(ctx, tt.key)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			cmpOpts := []cmp.Option{
				protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
				protocmp.Transform(),
			}
			require.Empty(t, cmp.Diff(tt.wantObj, obj, cmpOpts...))
		})
	}
}

func TestUpdateDatabaseObject(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getService(t)
	prepopulate(t, service, 1)

	expiry := timestamppb.New(time.Now().Add(30 * time.Minute))

	obj := getObject(t, 0)
	obj.Metadata.Expires = expiry

	objUpdated, err := service.UpdateDatabaseObject(ctx, obj)
	require.NoError(t, err)
	require.Equal(t, expiry, objUpdated.Metadata.Expires)

	objFresh, err := service.GetDatabaseObject(ctx, obj.Metadata.Name)
	require.NoError(t, err)
	require.Equal(t, expiry, objFresh.Metadata.Expires)
}

func TestDeleteDatabaseObject(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getService(t)
	prepopulate(t, service, 1)

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "object does not exist",
			key:     "dummy",
			wantErr: true,
		},
		{
			name:    "success",
			key:     getObject(t, 0).GetMetadata().GetName(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Fetch a specific object.
			err := service.DeleteDatabaseObject(ctx, tt.key)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestListDatabaseObjects(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	counts := []int{0, 1, 5, 10}
	for _, count := range counts {
		t.Run(fmt.Sprintf("count=%v", count), func(t *testing.T) {
			service := getService(t)
			prepopulate(t, service, count)

			t.Run("one page", func(t *testing.T) {
				// Fetch all objects.
				elements, nextToken, err := service.ListDatabaseObjects(ctx, 200, "")
				require.NoError(t, err)
				require.Empty(t, nextToken)
				require.Len(t, elements, count)

				for i := 0; i < count; i++ {
					cmpOpts := []cmp.Option{
						protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
						protocmp.Transform(),
					}
					require.Empty(t, cmp.Diff(getObject(t, i), elements[i], cmpOpts...))
				}
			})

			t.Run("paginated", func(t *testing.T) {
				// Fetch a paginated list of objects
				elements := make([]*dbobjectv1.DatabaseObject, 0)
				nextToken := ""
				for {
					out, token, err := service.ListDatabaseObjects(ctx, 2, nextToken)
					require.NoError(t, err)
					nextToken = token

					elements = append(elements, out...)
					if nextToken == "" {
						break
					}
				}

				for i := 0; i < count; i++ {
					cmpOpts := []cmp.Option{
						protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
						protocmp.Transform(),
					}
					require.Empty(t, cmp.Diff(getObject(t, i), elements[i], cmpOpts...))
				}
			})
		})
	}
}

func TestMarshalDatabaseObjectRoundTrip(t *testing.T) {
	t.Parallel()

	spec := &dbobjectv1.DatabaseObjectSpec{Name: "dummy", Protocol: "postgres"}
	obj, err := databaseobject.NewDatabaseObject("dummy-table", spec)
	require.NoError(t, err)

	//nolint:staticcheck // SA1019. Using this marshaler for json compatibility.
	out, err := services.FastMarshalProtoResourceDeprecated(obj)
	require.NoError(t, err)
	//nolint:staticcheck // SA1019. Using this unmarshaler for json compatibility.
	newObj, err := services.FastUnmarshalProtoResourceDeprecated[*dbobjectv1.DatabaseObject](out)
	require.NoError(t, err)
	require.True(t, proto.Equal(obj, newObj), "messages are not equal")
}
