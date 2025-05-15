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
}
