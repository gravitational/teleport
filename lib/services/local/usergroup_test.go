/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
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
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Fetch a specific user group.
	sp, err := service.GetUserGroup(ctx, g2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(g2, sp,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
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
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Delete a user group.
	err = service.DeleteUserGroup(ctx, g1.GetName())
	require.NoError(t, err)
	out, nextToken, err = service.ListUserGroups(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]types.UserGroup{g2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
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
