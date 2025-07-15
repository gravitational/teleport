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

package local_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestProvisioningService_ListProvisionTokens_Pagination(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cases := []struct {
		name     string
		count    int
		pageSize int
		nextPage string
	}{
		{
			name:     "empty",
			count:    10,
			pageSize: defaults.MaxIterationLimit,
			nextPage: "",
		},
		{
			name:     "count smaller than page size",
			count:    1,
			pageSize: 2,
			nextPage: "",
		},
		{
			name:     "count bigger than page size",
			count:    3,
			pageSize: 2,
			nextPage: "test-token-3",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service := newProvisioningService(t, clockwork.NewFakeClock())

			for j := range tc.count {
				err := service.CreateToken(ctx, &types.ProvisionTokenV2{
					Metadata: types.Metadata{
						Name: "test-token-" + strconv.Itoa(j+1),
					},
					Spec: types.ProvisionTokenSpecV2{
						Roles: []types.SystemRole{
							types.RoleAdmin,
						},
					},
				})
				require.NoError(t, err)
			}

			result, nextPage, err := service.ListProvisionTokens(ctx, tc.pageSize, "", nil, "")
			require.NoError(t, err)
			require.Equal(t, tc.nextPage, nextPage)
			if tc.count > tc.pageSize {
				require.Len(t, result, tc.pageSize)

				result, nextPage, err = service.ListProvisionTokens(ctx, tc.pageSize, nextPage, nil, "")
				require.NoError(t, err)

				require.Empty(t, nextPage)
				require.Len(t, result, tc.count-tc.pageSize)
			} else {
				require.Len(t, result, tc.count)
			}
		})
	}
}

func TestProvisioningService_ListProvisionTokens_FilterAnyRoles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	service := newProvisioningService(t, clockwork.NewFakeClock())

	tokens := []struct {
		roles   types.SystemRoles
		botName string
	}{
		{
			roles:   types.SystemRoles{types.RoleAdmin},
			botName: "",
		},
		{
			roles:   types.SystemRoles{types.RoleAdmin, types.RoleNode},
			botName: "",
		},
		{
			roles:   types.SystemRoles{types.RoleBot},
			botName: "bot-2",
		},
	}

	for i, token := range tokens {
		err := service.CreateToken(ctx, &types.ProvisionTokenV2{
			Metadata: types.Metadata{
				Name: "test-token-" + strconv.Itoa(i+1),
			},
			Spec: types.ProvisionTokenSpecV2{
				Roles:   token.roles,
				BotName: token.botName,
			},
		})
		require.NoError(t, err)
	}

	result, _, err := service.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", nil, "")
	require.NoError(t, err)
	require.Len(t, result, 3)

	result, _, err = service.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", types.SystemRoles{types.RoleAdmin, types.RoleNode, types.RoleBot}, "")
	require.NoError(t, err)
	require.Len(t, result, 3)

	result, _, err = service.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", types.SystemRoles{types.RoleAdmin, types.RoleNode}, "")
	require.NoError(t, err)
	require.Len(t, result, 2)

	result, _, err = service.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", types.SystemRoles{types.RoleBot}, "")
	require.NoError(t, err)
	require.Len(t, result, 1)
}

func TestProvisioningService_ListProvisionTokens_FilterBotName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	service := newProvisioningService(t, clockwork.NewFakeClock())

	tokens := []struct {
		roles   types.SystemRoles
		botName string
	}{
		{
			roles:   types.SystemRoles{types.RoleAdmin},
			botName: "",
		},
		{
			roles:   types.SystemRoles{types.RoleAdmin, types.RoleBot},
			botName: "bot-1",
		},
		{
			roles:   types.SystemRoles{types.RoleBot},
			botName: "bot-2",
		},
	}

	for i, token := range tokens {
		err := service.CreateToken(ctx, &types.ProvisionTokenV2{
			Metadata: types.Metadata{
				Name: "test-token-" + strconv.Itoa(i+1),
			},
			Spec: types.ProvisionTokenSpecV2{
				Roles:   token.roles,
				BotName: token.botName,
			},
		})
		require.NoError(t, err)
	}

	result, _, err := service.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", nil, "bot-1")
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "test-token-2", result[0].GetName())

	result, _, err = service.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", nil, "bot-2")
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "test-token-3", result[0].GetName())
}

func newProvisioningService(t *testing.T, clock clockwork.Clock) *local.ProvisioningService {
	t.Helper()
	backend, err := memory.New(memory.Config{
		Context: context.Background(),
		Clock:   clock,
	})
	require.NoError(t, err)
	service := local.NewProvisioningService(backend)
	return service
}
