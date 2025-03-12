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
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types/userprovisioning"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
)

func TestCreateStaticHostUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getStaticHostUserService(t)

	obj := getStaticHostUser(0)

	// first attempt should succeed
	objOut, err := service.CreateStaticHostUser(ctx, obj)
	require.NoError(t, err)
	require.Equal(t, obj, objOut)

	// second attempt should fail, object already exists
	_, err = service.CreateStaticHostUser(ctx, obj)
	require.True(t, trace.IsAlreadyExists(err))
}

func TestUpsertStaticHostUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getStaticHostUserService(t)

	obj := getStaticHostUser(0)

	// first attempt should succeed
	objOut, err := service.UpsertStaticHostUser(ctx, obj)
	require.NoError(t, err)
	require.Equal(t, obj, objOut)

	// second attempt should also succeed
	objOut, err = service.UpsertStaticHostUser(ctx, obj)
	require.NoError(t, err)
	require.Equal(t, obj, objOut)
}

func TestGetStaticHostUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getStaticHostUserService(t)
	prepopulateStaticHostUsers(t, service, 1)

	tests := []struct {
		name      string
		key       string
		assertErr assert.ErrorAssertionFunc
		wantObj   *userprovisioningpb.StaticHostUser
	}{
		{
			name: "object does not exist",
			key:  "dummy",
			assertErr: func(t assert.TestingT, err error, msgAndArgs ...interface{}) bool {
				return assert.True(t, trace.IsNotFound(err), msgAndArgs...)
			},
		},
		{
			name:      "success",
			key:       getStaticHostUser(0).GetMetadata().GetName(),
			assertErr: assert.NoError,
			wantObj:   getStaticHostUser(0),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			obj, err := service.GetStaticHostUser(ctx, tc.key)
			tc.assertErr(t, err)
			if tc.wantObj == nil {
				assert.Nil(t, obj)
			} else {
				cmpOpts := []cmp.Option{
					protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
					protocmp.Transform(),
				}
				require.Equal(t, "", cmp.Diff(tc.wantObj, obj, cmpOpts...))
			}
		})
	}
}

func TestUpdateStaticHostUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getStaticHostUserService(t)
	prepopulateStaticHostUsers(t, service, 1)

	expiry := timestamppb.New(time.Now().Add(30 * time.Minute))

	// Fetch the object from the backend so the revision is populated.
	key := getStaticHostUser(0).GetMetadata().GetName()
	obj, err := service.GetStaticHostUser(ctx, key)
	require.NoError(t, err)
	obj.Metadata.Expires = expiry

	objUpdated, err := service.UpdateStaticHostUser(ctx, obj)
	require.NoError(t, err)
	require.Equal(t, expiry, objUpdated.Metadata.Expires)

	objFresh, err := service.GetStaticHostUser(ctx, key)
	require.NoError(t, err)
	require.Equal(t, expiry, objFresh.Metadata.Expires)
}

func TestUpdateStaticHostUserMissingRevision(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getStaticHostUserService(t)
	prepopulateStaticHostUsers(t, service, 1)

	expiry := timestamppb.New(time.Now().Add(30 * time.Minute))

	obj := getStaticHostUser(0)
	obj.Metadata.Expires = expiry

	// Update should be rejected as the revision is missing.
	_, err := service.UpdateStaticHostUser(ctx, obj)
	require.True(t, trace.IsCompareFailed(err))
}

func TestDeleteStaticHostUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := getStaticHostUserService(t)
	prepopulateStaticHostUsers(t, service, 1)

	tests := []struct {
		name      string
		key       string
		assertErr require.ErrorAssertionFunc
	}{
		{
			name: "object does not exist",
			key:  "dummy",
			assertErr: func(t require.TestingT, err error, msgAndArgs ...interface{}) {
				require.True(t, trace.IsNotFound(err), msgAndArgs...)
			},
		},
		{
			name:      "success",
			key:       getStaticHostUser(0).GetMetadata().GetName(),
			assertErr: require.NoError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := service.DeleteStaticHostUser(ctx, tc.key)
			tc.assertErr(t, err)
		})
	}
}

func TestListStaticHostUsers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	counts := []int{0, 1, 5, 10}

	for _, count := range counts {
		t.Run(fmt.Sprintf("count=%v", count), func(t *testing.T) {
			service := getStaticHostUserService(t)
			prepopulateStaticHostUsers(t, service, count)

			t.Run("one page", func(t *testing.T) {
				// Fetch all objects.
				elements, nextToken, err := service.ListStaticHostUsers(ctx, 200, "")
				require.NoError(t, err)
				require.Empty(t, nextToken)
				require.Len(t, elements, count)

				for i := 0; i < count; i++ {
					cmpOpts := []cmp.Option{
						protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
						protocmp.Transform(),
					}
					require.Equal(t, "", cmp.Diff(getStaticHostUser(i), elements[i], cmpOpts...))
				}
			})

			t.Run("paginated", func(t *testing.T) {
				// Fetch a paginated list of objects
				elements := make([]*userprovisioningpb.StaticHostUser, 0)
				nextToken := ""
				for {
					out, token, err := service.ListStaticHostUsers(ctx, 2, nextToken)
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
					require.Equal(t, "", cmp.Diff(getStaticHostUser(i), elements[i], cmpOpts...))
				}
			})
		})
	}
}

func getStaticHostUserService(t *testing.T) services.StaticHostUser {
	backend, err := memory.New(memory.Config{
		Context: context.Background(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service, err := NewStaticHostUserService(backend)
	require.NoError(t, err)
	return service
}

func getStaticHostUser(index int) *userprovisioningpb.StaticHostUser {
	name := fmt.Sprintf("obj%v", index)
	return userprovisioning.NewStaticHostUser(name, &userprovisioningpb.StaticHostUserSpec{
		Matchers: []*userprovisioningpb.Matcher{
			{
				NodeLabels: []*labelv1.Label{
					{
						Name:   "foo",
						Values: []string{"bar"},
					},
				},
				Groups: []string{"foo", "bar"},
				Uid:    1234,
				Gid:    5678,
			},
		},
	})
}

func prepopulateStaticHostUsers(t *testing.T, service services.StaticHostUser, count int) {
	for i := 0; i < count; i++ {
		_, err := service.CreateStaticHostUser(context.Background(), getStaticHostUser(i))
		require.NoError(t, err)
	}
}
