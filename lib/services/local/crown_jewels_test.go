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

package local_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/lib/auth/crownjewel"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestCreateCrownJewel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getService(t)

	obj, err := crownjewel.NewCrownJewel("obj", &crownjewelv1.CrownJewelSpec{
		TeleportMatchers: []*crownjewelv1.TeleportMatcher{
			{
				Kinds: []string{"node"},
				Names: []string{"test"},
			},
		},
	})
	require.NoError(t, err)

	// first attempt should succeed
	objOut, err := service.CreateCrownJewel(ctx, obj)
	require.NoError(t, err)
	require.Equal(t, obj, objOut)

	// second attempt should fail, object already exists
	_, err = service.CreateCrownJewel(ctx, obj)
	require.Error(t, err)
}

func TestUpsertCrownJewel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getService(t)

	obj, err := crownjewel.NewCrownJewel("obj", &crownjewelv1.CrownJewelSpec{
		TeleportMatchers: []*crownjewelv1.TeleportMatcher{
			{
				Kinds: []string{"node"},
				Names: []string{"test"},
			},
		},
	})
	require.NoError(t, err)

	// the first attempt should succeed
	objOut, err := service.UpsertCrownJewel(ctx, obj)
	require.NoError(t, err)
	require.Equal(t, obj, objOut)

	// the second attempt should also succeed
	objOut, err = service.UpsertCrownJewel(ctx, obj)
	require.NoError(t, err)
	require.Equal(t, obj, objOut)
}

func TestGetCrownJewel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getService(t)
	prepopulate(t, service, 1)

	tests := []struct {
		name    string
		key     string
		wantErr bool
		wantObj *crownjewelv1.CrownJewel
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
			obj, err := service.GetCrownJewel(ctx, tt.key)
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

func TestUpdateCrownJewel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getService(t)
	prepopulate(t, service, 1)

	expiry := timestamppb.New(time.Now().Add(30 * time.Minute))

	// Fetch the object from the backend so the revision is populated.
	obj, err := service.GetCrownJewel(ctx, getObject(t, 0).GetMetadata().GetName())
	require.NoError(t, err)
	// update the expiry time
	obj.Metadata.Expires = expiry

	objUpdated, err := service.UpdateCrownJewel(ctx, obj)
	require.NoError(t, err)
	require.Equal(t, expiry, objUpdated.Metadata.Expires)

	objFresh, err := service.GetCrownJewel(ctx, obj.Metadata.Name)
	require.NoError(t, err)
	require.Equal(t, expiry, objFresh.Metadata.Expires)
}

func TestUpdateCrownJewelMissingRevision(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getService(t)
	prepopulate(t, service, 1)

	expiry := timestamppb.New(time.Now().Add(30 * time.Minute))

	obj := getObject(t, 0)
	obj.Metadata.Expires = expiry

	// Update should be rejected as the revision is missing.
	_, err := service.UpdateCrownJewel(ctx, obj)
	require.Error(t, err)
}

func TestDeleteCrownJewel(t *testing.T) {
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
			err := service.DeleteCrownJewel(ctx, tt.key)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestListCrownJewel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	counts := []int{0, 1, 5, 10}
	for _, count := range counts {
		t.Run(fmt.Sprintf("count=%v", count), func(t *testing.T) {
			service := getService(t)
			prepopulate(t, service, count)

			t.Run("one page", func(t *testing.T) {
				// Fetch all objects.
				elements, nextToken, err := service.ListCrownJewels(ctx, 200, "")
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
				elements := make([]*crownjewelv1.CrownJewel, 0)
				nextToken := ""
				for {
					out, token, err := service.ListCrownJewels(ctx, 2, nextToken)
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

func getService(t *testing.T) services.CrownJewels {
	backend, err := memory.New(memory.Config{
		Context: context.Background(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service, err := local.NewCrownJewelsService(backend)
	require.NoError(t, err)
	return service
}

func getObject(t *testing.T, index int) *crownjewelv1.CrownJewel {
	name := fmt.Sprintf("obj%v", index)
	obj, err := crownjewel.NewCrownJewel(name, &crownjewelv1.CrownJewelSpec{
		TeleportMatchers: []*crownjewelv1.TeleportMatcher{
			{
				Kinds: []string{"node"},
				Names: []string{"test"},
			},
		},
	})
	require.NoError(t, err)

	return obj
}

func prepopulate(t *testing.T, service services.CrownJewels, count int) {
	for i := 0; i < count; i++ {
		_, err := service.CreateCrownJewel(context.Background(), getObject(t, i))
		require.NoError(t, err)
	}
}
