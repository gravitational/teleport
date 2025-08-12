/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func setupIdentityTest(t *testing.T) *IdentityService {
	clock := clockwork.NewFakeClock()
	mem, err := memory.New(memory.Config{
		Context: t.Context(),
		Clock:   clock,
	})
	require.NoError(t, err)
	service, err := NewIdentityService(backend.NewSanitizer(mem))
	require.NoError(t, err)
	return service
}

func TestIdentityService_ListUserTokens(t *testing.T) {
	t.Parallel()
	t.Run("none", func(t *testing.T) {
		ctx := t.Context()
		service := setupIdentityTest(t)
		userTokens, _, err := service.ListUserTokens(ctx, 5, "")
		require.NoError(t, err)
		require.Empty(t, userTokens)
	})
	t.Run("list all", func(t *testing.T) {
		service := setupIdentityTest(t)
		ctx := t.Context()
		userToken1, err := service.CreateUserToken(ctx, &types.UserTokenV3{
			Kind:    types.KindUserToken,
			Version: types.V3,
			Metadata: types.Metadata{
				Name: "token-user-1",
			},
			Spec: types.UserTokenSpecV3{
				User:    "user1",
				Created: time.Now(),
			},
		})
		require.NoError(t, err)
		userToken2, err := service.CreateUserToken(ctx, &types.UserTokenV3{
			Kind:    types.KindUserToken,
			Version: types.V3,
			Metadata: types.Metadata{
				Name: "token-user-2",
			},
			Spec: types.UserTokenSpecV3{
				User:    "user2",
				Created: time.Now(),
			},
		})
		require.NoError(t, err)
		userTokens, next, err := service.ListUserTokens(ctx, 5, "")
		require.NoError(t, err)
		require.Len(t, userTokens, 2)
		require.Empty(t, next)
		require.Empty(t, cmp.Diff(
			userToken1,
			userTokens[0],
			protocmp.Transform(),
		))
		require.Empty(t, cmp.Diff(
			userToken2,
			userTokens[1],
			protocmp.Transform(),
		))
	})
	t.Run("list paged", func(t *testing.T) {
		service := setupIdentityTest(t)
		ctx := t.Context()
		userToken1, err := service.CreateUserToken(ctx, &types.UserTokenV3{
			Kind:    types.KindUserToken,
			Version: types.V3,
			Metadata: types.Metadata{
				Name: "token-user-1",
			},
			Spec: types.UserTokenSpecV3{
				User:    "user1",
				Created: time.Now(),
			},
		})
		require.NoError(t, err)
		userToken2, err := service.CreateUserToken(ctx, &types.UserTokenV3{
			Kind:    types.KindUserToken,
			Version: types.V3,
			Metadata: types.Metadata{
				Name: "token-user-2",
			},
			Spec: types.UserTokenSpecV3{
				User:    "user2",
				Created: time.Now(),
			},
		})
		require.NoError(t, err)
		userTokens, next, err := service.ListUserTokens(ctx, 1, "")
		t.Log(userTokens)
		t.Log(next)
		require.NoError(t, err)
		require.Len(t, userTokens, 1)
		require.NotEmpty(t, next)
		require.Empty(t, cmp.Diff(
			userToken1,
			userTokens[0],
			protocmp.Transform(),
		))
		userTokens, next, err = service.ListUserTokens(ctx, 1, next)
		t.Log(userTokens)
		t.Log(next)
		require.NoError(t, err)
		require.Len(t, userTokens, 1)
		require.Empty(t, next)
		require.Empty(t, cmp.Diff(
			userToken2,
			userTokens[0],
			protocmp.Transform(),
		))
	})
}
