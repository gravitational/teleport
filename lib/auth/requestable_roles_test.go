/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package auth

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	authpb "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/tlsca"
)

// TestListRequestableRoles tests listing requestable roles with pagination.
func TestListRequestableRoles(t *testing.T) {
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)
	a := p.a

	username := "test-user"
	userRole, err := types.NewRole("user-role", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Roles: []string{"role-*"}, // Allow requesting all roles that start with "role-"
			},
		},
	})
	require.NoError(t, err)

	_, err = a.CreateRole(ctx, userRole)
	require.NoError(t, err)

	user, err := types.NewUser(username)
	require.NoError(t, err)
	user.SetRoles([]string{"user-role"})

	_, err = a.CreateUser(ctx, user)
	require.NoError(t, err)

	// Create dummy requestable roles
	roleNames := []string{
		"role-1", "role-2", "role-3", "role-4",
		"role-5", "role-6", "role-7", "role-8",
		"role-9", "role-99",
	}

	var expectedRequestableRoles []string
	for _, roleName := range roleNames {
		role, err := types.NewRole(roleName, types.RoleSpecV6{
			Allow: types.RoleConditions{
				Logins: []string{"ubuntu"},
			},
		})
		require.NoError(t, err)

		_, err = a.CreateRole(ctx, role)
		require.NoError(t, err)
		expectedRequestableRoles = append(expectedRequestableRoles, roleName)
	}

	nonRequestableRole, err := types.NewRole("admin", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{"root"},
		},
	})
	require.NoError(t, err)
	_, err = a.CreateRole(ctx, nonRequestableRole)
	require.NoError(t, err)

	userIdentity := tlsca.Identity{
		Username: username,
		Groups:   []string{"user-role"},
	}
	userCtx := authz.ContextWithUser(ctx, authz.LocalUser{
		Username: username,
		Identity: userIdentity,
	})

	t.Run("list all requestable roles, no limit", func(t *testing.T) {
		req := &authpb.ListRequestableRolesRequest{}
		resp, err := a.ListRequestableRoles(userCtx, req)
		require.NoError(t, err)

		// Verify that all the requestable roles were returned.
		var receivedRoles []string
		for _, role := range resp.Roles {
			receivedRoles = append(receivedRoles, role.GetName())
		}
		require.Empty(t, cmp.Diff(expectedRequestableRoles, receivedRoles))

		// Verify that the non-requestable roles aren't returned.
		require.NotContains(t, receivedRoles, "admin")
		require.NotContains(t, receivedRoles, "user-role")

		// There shouldn't be a nextKey
		require.Empty(t, resp.NextKey)
	})

	t.Run("list a page of requestable roles starting with a startKey", func(t *testing.T) {
		// Get the first page of 3
		firstPageReq := &authpb.ListRequestableRolesRequest{Limit: 3}
		firstPageResp, err := a.ListRequestableRoles(userCtx, firstPageReq)
		require.NoError(t, err)
		require.NotEmpty(t, firstPageResp.NextKey)

		secondPageReq := &authpb.ListRequestableRolesRequest{
			Limit:    3,
			StartKey: firstPageResp.NextKey,
		}
		secondPageResp, err := a.ListRequestableRoles(userCtx, secondPageReq)
		require.NoError(t, err)

		// Verify that the second page has the correct roles.
		var receivedRoles []string
		for _, role := range secondPageResp.Roles {
			receivedRoles = append(receivedRoles, role.GetName())
		}
		require.Empty(t, cmp.Diff(expectedRequestableRoles[3:6], receivedRoles))

		// Verify there is no overlap in roles between the pages.
		firstPageRoleNames := make(map[string]bool)
		for _, role := range firstPageResp.Roles {
			firstPageRoleNames[role.GetName()] = true
		}
		for _, role := range secondPageResp.Roles {
			require.False(t, firstPageRoleNames[role.GetName()])
		}
	})

	t.Run("list all pages of requestable roles", func(t *testing.T) {
		limit := int32(3)
		var respRoles []*types.RoleV6
		nextKey := ""

		for {
			req := &authpb.ListRequestableRolesRequest{
				Limit:    limit,
				StartKey: nextKey,
			}

			resp, err := a.ListRequestableRoles(userCtx, req)
			require.NoError(t, err)

			respRoles = append(respRoles, resp.Roles...)

			if resp.NextKey == "" {
				break
			}

			// Verify that we got the correct page size.
			if resp.NextKey != "" {
				require.Len(t, resp.Roles, int(limit))
			}

			nextKey = resp.NextKey
		}

		// Verify that all the requestable roles were returned.
		var receivedRoles []string
		for _, role := range respRoles {
			receivedRoles = append(receivedRoles, role.GetName())
		}
		require.Empty(t, cmp.Diff(expectedRequestableRoles, receivedRoles))
	})

	t.Run("no requestable roles", func(t *testing.T) {
		// Create a user with no requestable roles
		noPermsUsername := "noperms-user"
		noPermsRole, err := types.NewRole("noperms-role", types.RoleSpecV6{
			Allow: types.RoleConditions{
				Logins: []string{"ubuntu"},
			},
			// No request permissions.
		})
		require.NoError(t, err)

		_, err = a.CreateRole(ctx, noPermsRole)
		require.NoError(t, err)

		noPermsUser, err := types.NewUser(noPermsUsername)
		require.NoError(t, err)
		noPermsUser.SetRoles([]string{"noperms-role"})

		_, err = a.CreateUser(ctx, noPermsUser)
		require.NoError(t, err)

		noPermsIdentity := tlsca.Identity{
			Username: noPermsUsername,
			Groups:   []string{"noperms-role"},
		}
		restrictedCtx := authz.ContextWithUser(ctx, authz.LocalUser{
			Username: noPermsUsername,
			Identity: noPermsIdentity,
		})

		req := &authpb.ListRequestableRolesRequest{}
		resp, err := a.ListRequestableRoles(restrictedCtx, req)
		require.NoError(t, err)

		// Verify that nothing is returned.
		require.Empty(t, resp.Roles)
		require.Empty(t, resp.NextKey)
	})

	t.Run("list requestable roles with a search filter", func(t *testing.T) {
		req := &authpb.ListRequestableRolesRequest{
			Filter: &types.RoleFilter{
				SearchKeywords: []string{"role-99"},
			},
		}
		resp, err := a.ListRequestableRoles(userCtx, req)
		require.NoError(t, err)

		// Verify that only "role-99" was returned
		require.Len(t, resp.Roles, 1)
		require.Equal(t, resp.Roles[0].GetName(), "role-99")

		// There shouldn't be a nextKey
		require.Empty(t, resp.NextKey)
	})

}
