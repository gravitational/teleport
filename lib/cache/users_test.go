// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
)

// TestUsers tests caching of users
func TestUsers(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	t.Run("GetUsers", func(t *testing.T) {
		testResources(t, p, testFuncs[types.User]{
			newResource: func(name string) (types.User, error) {
				return types.NewUser(name)
			},
			create: func(ctx context.Context, user types.User) error {
				_, err := p.usersS.UpsertUser(ctx, user)
				return err
			},
			list:      getAllAdapter(func(ctx context.Context) ([]types.User, error) { return p.usersS.GetUsers(ctx, false) }),
			cacheList: getAllAdapter(func(ctx context.Context) ([]types.User, error) { return p.cache.GetUsers(ctx, false) }),
			update: func(ctx context.Context, user types.User) error {
				_, err := p.usersS.UpdateUser(ctx, user)
				return err
			},
			deleteAll: p.usersS.DeleteAllUsers,
		}, withSkipPaginationTest())
	})

	t.Run("ListUsers", func(t *testing.T) {
		testResources(t, p, testFuncs[types.User]{
			newResource: func(name string) (types.User, error) {
				return types.NewUser(name)
			},
			create: func(ctx context.Context, user types.User) error {
				_, err := p.usersS.UpsertUser(ctx, user)
				return err
			},
			list: func(ctx context.Context, pageSize int, pageToken string) ([]types.User, string, error) {
				var out []types.User
				req := &userspb.ListUsersRequest{
					PageSize:  int32(pageSize),
					PageToken: pageToken,
				}
				resp, err := p.usersS.ListUsers(ctx, req)
				if err != nil {
					return nil, "", trace.Wrap(err)
				}

				for _, u := range resp.Users {
					out = append(out, u)
				}

				return out, resp.NextPageToken, nil
			},
			cacheList: func(ctx context.Context, pageSize int, pageToken string) ([]types.User, string, error) {
				var out []types.User
				req := &userspb.ListUsersRequest{
					PageSize:  int32(pageSize),
					PageToken: pageToken,
				}
				resp, err := p.cache.ListUsers(ctx, req)
				if err != nil {
					return nil, "", trace.Wrap(err)
				}

				for _, u := range resp.Users {
					out = append(out, u)
				}

				return out, resp.NextPageToken, nil
			},
			update: func(ctx context.Context, user types.User) error {
				_, err := p.usersS.UpdateUser(ctx, user)
				return err
			},
			deleteAll: func(ctx context.Context) error {
				return p.usersS.DeleteAllUsers(ctx)
			},
		})
	})

	t.Run("ListUsers pagination", func(t *testing.T) {
		ctx := t.Context()

		err := p.usersS.DeleteAllUsers(t.Context())
		require.NoError(t, err)

		waitForUsersCacheLen := func(expected int) {
			require.EventuallyWithT(t, func(t *assert.CollectT) {
				got, err := p.cache.GetUsers(ctx, false)
				require.NoError(t, err)
				require.Len(t, got, expected)
			}, 3*time.Second, time.Millisecond*100)
		}
		waitForUsersCacheLen(0)

		usersToCreate := []string{"bob", "alice"}
		for _, userName := range usersToCreate {
			user, err := types.NewUser(userName)
			require.NoError(t, err)
			_, err = p.usersS.UpsertUser(t.Context(), user)
			require.NoError(t, err)
		}

		waitForUsersCacheLen(len(usersToCreate))

		var allUsers []*types.UserV2
		req := &userspb.ListUsersRequest{
			PageSize: 1,
		}
		for {
			resp, err := p.cache.ListUsers(t.Context(), req)
			require.NoError(t, err)
			require.Len(t, resp.Users, 1)

			allUsers = append(allUsers, resp.Users...)
			if resp.NextPageToken == "" {
				break
			}
			req.PageToken = resp.NextPageToken
		}
		require.Len(t, allUsers, len(usersToCreate))
	})
}
