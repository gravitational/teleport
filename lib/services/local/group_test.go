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

// TestGroupCRUD tests backend operations with group resources.
func TestGroupCRUD(t *testing.T) {
	ctx := context.Background()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	service := NewGroupService(backend)

	// Create a couple groups.
	g1, err := types.NewGroup(
		types.Metadata{
			Name: "g1",
		},
	)
	require.NoError(t, err)
	g2, err := types.NewGroup(
		types.Metadata{
			Name: "g2",
		},
	)
	require.NoError(t, err)

	// Initially we expect no groups.
	out, nextToken, err := service.ListGroups(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)

	// Create both groups.
	err = service.CreateGroup(ctx, g1)
	require.NoError(t, err)
	err = service.CreateGroup(ctx, g2)
	require.NoError(t, err)

	// Fetch all groups.
	out, nextToken, err = service.ListGroups(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]types.Group{g1, g2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Fetch a paginated list of groups.
	paginatedOut := make([]types.Group, 0, 2)
	numPages := 0
	for {
		numPages++
		out, nextToken, err = service.ListGroups(ctx, 1, nextToken)
		require.NoError(t, err)

		paginatedOut = append(paginatedOut, out...)
		if nextToken == "" {
			break
		}
	}

	require.Equal(t, 2, numPages)
	require.Empty(t, cmp.Diff([]types.Group{g1, g2}, paginatedOut,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Fetch a specific group.
	sp, err := service.GetGroup(ctx, g2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(g2, sp,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to fetch a group that doesn't exist.
	_, err = service.GetGroup(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Try to create the same group.
	err = service.CreateGroup(ctx, g1)
	require.IsType(t, trace.AlreadyExists(""), err)

	// Update a group.
	g1.SetOrigin(types.OriginCloud)
	err = service.UpdateGroup(ctx, g1)
	require.NoError(t, err)
	sp, err = service.GetGroup(ctx, g1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(g1, sp,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Delete a group.
	err = service.DeleteGroup(ctx, g1.GetName())
	require.NoError(t, err)
	out, nextToken, err = service.ListGroups(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, cmp.Diff([]types.Group{g2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to delete a group that doesn't exist.
	err = service.DeleteGroup(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err))

	// Delete all groups.
	err = service.DeleteAllGroups(ctx)
	require.NoError(t, err)
	out, nextToken, err = service.ListGroups(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextToken)
	require.Empty(t, out)
}
