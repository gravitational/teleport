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
)

// TestUserGroupCRUD tests backend operations with user group resources.
func TestUserGroupCRUD(t *testing.T) {
	ctx := context.Background()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service, err := NewUserGroupService(backend)
	require.NoError(t, err)

	// Create a couple user groups.
	g1, err := types.NewUserGroup(
		types.Metadata{
			Name: "g1",
		}, types.UserGroupSpecV1{})
	require.NoError(t, err)
	g2, err := types.NewUserGroup(
		types.Metadata{
			Name: "g2",
		}, types.UserGroupSpecV1{})
	require.NoError(t, err)

	// Initially we expect no user groups.
	out, nextToken, err := service.ListUserGroups(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)

	// Create both user groups.
	err = service.CreateUserGroup(ctx, g1)
	require.NoError(t, err)
	err = service.CreateUserGroup(ctx, g2)
	require.NoError(t, err)

	// Fetch all user groups.
	out, nextToken, err = service.ListUserGroups(ctx, 0, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]types.UserGroup{g1, g2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Fetch a paginated list of user groups.
	paginatedOut := make([]types.UserGroup, 0, 2)
	numPages := 0
	for {
		numPages++
		out, nextToken, err = service.ListUserGroups(ctx, 1, nextToken)
		require.NoError(t, err)

		paginatedOut = append(paginatedOut, out...)
		if nextToken == "" {
			break
		}
	}

	require.Equal(t, 2, numPages)
	require.Empty(t, cmp.Diff([]types.UserGroup{g1, g2}, paginatedOut,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Fetch a specific user group.
	sp, err := service.GetUserGroup(ctx, g2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(g2, sp,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Try to fetch a user group that doesn't exist.
	_, err = service.GetUserGroup(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err))

	// Try to create the same user group.
	err = service.CreateUserGroup(ctx, g1)
	require.True(t, trace.IsAlreadyExists(err))

	// Update a user group.
	g1.SetOrigin(types.OriginCloud)
	err = service.UpdateUserGroup(ctx, g1)
	require.NoError(t, err)
	sp, err = service.GetUserGroup(ctx, g1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(g1, sp,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Delete a user group.
	err = service.DeleteUserGroup(ctx, g1.GetName())
	require.NoError(t, err)
	out, nextToken, err = service.ListUserGroups(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]types.UserGroup{g2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
	))

	// Try to delete a user group that doesn't exist.
	err = service.DeleteUserGroup(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err))

	// Delete all user groups.
	err = service.DeleteAllUserGroups(ctx)
	require.NoError(t, err)
	out, nextToken, err = service.ListUserGroups(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)
}
