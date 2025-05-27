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
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// TestUserGroups tests that CRUD operations on user group resources are
// replicated from the backend to the cache.
func TestUserGroups(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.UserGroup]{
		newResource: func(name string) (types.UserGroup, error) {
			return types.NewUserGroup(
				types.Metadata{
					Name: name,
				}, types.UserGroupSpecV1{},
			)
		},
		create: p.userGroups.CreateUserGroup,
		list: func(ctx context.Context) ([]types.UserGroup, error) {
			results, _, err := p.userGroups.ListUserGroups(ctx, 0, "")
			return results, err
		},
		cacheGet: p.cache.GetUserGroup,
		cacheList: func(ctx context.Context) ([]types.UserGroup, error) {
			results, _, err := p.cache.ListUserGroups(ctx, 0, "")
			return results, err
		},
		update:    p.userGroups.UpdateUserGroup,
		deleteAll: p.userGroups.DeleteAllUserGroups,
	})

	require.NoError(t, p.userGroups.DeleteAllUserGroups(t.Context()))

	var expected []types.UserGroup
	for i := 0; i < 500; i++ {
		ug, err := types.NewUserGroup(
			types.Metadata{
				Name: "ug-" + strconv.Itoa(i+1),
			}, types.UserGroupSpecV1{})
		require.NoError(t, err)

		require.NoError(t, p.userGroups.CreateUserGroup(t.Context(), ug))

		ug, err = p.userGroups.GetUserGroup(t.Context(), ug.GetName())
		require.NoError(t, err)
		expected = append(expected, ug)
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.Equal(t, 500, p.cache.collections.userGroups.store.len())
	}, 10*time.Second, 100*time.Millisecond)

	var out []types.UserGroup
	var start string
	for {
		g, next, err := p.cache.ListUserGroups(t.Context(), 0, start)
		require.NoError(t, err)

		out = append(out, g...)
		if next == "" {
			break
		}

		// The /<token> here is to test an edge case that cause infinite listing. The
		// gPRC layer injected a / in the response which is not consumed by the new
		// cache collection, but was handled by the legacy local service.
		next = "/" + next
		start = next
	}

	assert.Len(t, out, len(expected))
	require.Empty(t, cmp.Diff(expected, out, cmpopts.SortSlices(func(x, y types.UserGroup) bool { return x.GetName() < y.GetName() })))

	out = nil
	start = ""
	for {
		g, next, err := p.cache.ListUserGroups(t.Context(), 0, start)
		require.NoError(t, err)

		out = append(out, g...)
		if next == "" {
			break
		}

		start = next
	}

	assert.Len(t, out, len(expected))
	require.Empty(t, cmp.Diff(expected, out, cmpopts.SortSlices(func(x, y types.UserGroup) bool { return x.GetName() < y.GetName() })))
}
