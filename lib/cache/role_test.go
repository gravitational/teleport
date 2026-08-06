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

	"github.com/gravitational/teleport/api/types"
)

func TestRoleNotFound(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForNode)
	t.Cleanup(p.Close)

	_, err := p.cache.GetRole(t.Context(), "test-role")
	assert.Error(t, err)
	assert.True(t, trace.IsNotFound(err))
	assert.Equal(t, "role test-role is not found", err.Error())
}

// TestRoles tests caching of roles
func TestRoles(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForNode)
	t.Cleanup(p.Close)

	newRole := func(name string) (types.Role, error) {
		return types.NewRole(name, types.RoleSpecV6{
			Options: types.RoleOptions{
				MaxSessionTTL: types.Duration(time.Hour),
			},
			Allow: types.RoleConditions{
				Logins:     []string{"root", "bob"},
				NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			},
			Deny: types.RoleConditions{},
		})
	}

	t.Run("GetRoles", func(t *testing.T) {
		testResources(t, p, testFuncs[types.Role]{
			newResource: newRole,
			create: func(ctx context.Context, role types.Role) error {
				_, err := p.accessS.UpsertRole(ctx, role)
				return err
			},
			list:      getAllAdapter(p.accessS.GetRoles),
			cacheGet:  p.cache.GetRole,
			cacheList: getAllAdapter(p.cache.GetRoles),
			update: func(ctx context.Context, role types.Role) error {
				_, err := p.accessS.UpsertRole(ctx, role)
				return err
			},
			deleteAll: p.accessS.DeleteAllRoles,
		}, withSkipPaginationTest())
	})

	t.Run("ListRoles", func(t *testing.T) {
		testRegisteredResources(
			t, p, registeredRoleCacheSpec(), p.accessS, p.cache,
			newRole,
			p.accessS.DeleteAllRoles,
		)
	})

}
