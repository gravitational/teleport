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

	"github.com/gravitational/teleport/api/client/proto"
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

	t.Run("GetRoles", func(t *testing.T) {
		testResources(t, p, testFuncs[types.Role]{
			newResource: func(name string) (types.Role, error) {
				return types.NewRole("role1", types.RoleSpecV6{
					Options: types.RoleOptions{
						MaxSessionTTL: types.Duration(time.Hour),
					},
					Allow: types.RoleConditions{
						Logins:     []string{"root", "bob"},
						NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					},
					Deny: types.RoleConditions{},
				})
			},
			create: func(ctx context.Context, role types.Role) error {
				_, err := p.accessS.UpsertRole(ctx, role)
				return err
			},
			list:      p.accessS.GetRoles,
			cacheGet:  p.cache.GetRole,
			cacheList: p.cache.GetRoles,
			update: func(ctx context.Context, role types.Role) error {
				_, err := p.accessS.UpsertRole(ctx, role)
				return err
			},
			deleteAll: func(ctx context.Context) error {
				return p.accessS.DeleteAllRoles(ctx)
			},
		})
	})

	t.Run("ListRoles", func(t *testing.T) {
		testResources(t, p, testFuncs[types.Role]{
			newResource: func(name string) (types.Role, error) {
				return types.NewRole("role1", types.RoleSpecV6{
					Options: types.RoleOptions{
						MaxSessionTTL: types.Duration(time.Hour),
					},
					Allow: types.RoleConditions{
						Logins:     []string{"root", "bob"},
						NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					},
					Deny: types.RoleConditions{},
				})
			},
			create: func(ctx context.Context, role types.Role) error {
				_, err := p.accessS.UpsertRole(ctx, role)
				return err
			},
			list: func(ctx context.Context) ([]types.Role, error) {
				var out []types.Role
				req := &proto.ListRolesRequest{}
				for {
					resp, err := p.accessS.ListRoles(ctx, req)
					if err != nil {
						return nil, trace.Wrap(err)
					}

					for _, r := range resp.Roles {
						out = append(out, r)
					}

					req.StartKey = resp.NextKey
					if resp.NextKey == "" {
						break
					}
				}

				return out, nil
			},
			cacheGet: p.cache.GetRole,
			cacheList: func(ctx context.Context) ([]types.Role, error) {
				var out []types.Role
				req := &proto.ListRolesRequest{}
				for {
					resp, err := p.cache.ListRoles(ctx, req)
					if err != nil {
						return nil, trace.Wrap(err)
					}

					for _, r := range resp.Roles {
						out = append(out, r)
					}

					req.StartKey = resp.NextKey
					if resp.NextKey == "" {
						break
					}
				}

				return out, nil
			},
			update: func(ctx context.Context, role types.Role) error {
				_, err := p.accessS.UpsertRole(ctx, role)
				return err
			},
			deleteAll: func(ctx context.Context) error {
				return p.accessS.DeleteAllRoles(ctx)
			},
		})
	})

}
