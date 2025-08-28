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

package auth_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	authpb "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/tlsca"
)

// TestListRequestableRoles tests listing requestable roles with pagination.
func TestListRequestableRoles(t *testing.T) {
	ctx := t.Context()
	tlsServer := newTestTLSServer(t)
	a := tlsServer.Auth()

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

	client, err := tlsServer.NewClient(authtest.TestUser(username))
	require.NoError(t, err)
	defer client.Close()

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

		resp, err := client.ListRequestableRoles(userCtx, req)
		require.NoError(t, err)

		// Verify that all the requestable roles were returned.
		var receivedRoles []string
		for _, role := range resp.Roles {
			receivedRoles = append(receivedRoles, role.Name)
		}
		require.Empty(t, cmp.Diff(expectedRequestableRoles, receivedRoles))

		// Verify that the non-requestable roles aren't returned.
		require.NotContains(t, receivedRoles, "admin")
		require.NotContains(t, receivedRoles, "user-role")

		// There shouldn't be a nextKey
		require.Empty(t, resp.NextPageToken)
	})

	t.Run("list a page of requestable roles starting with a startKey", func(t *testing.T) {
		// Get the first page of 3
		firstPageReq := &authpb.ListRequestableRolesRequest{
			PageSize: 3,
		}
		firstPageResp, err := client.ListRequestableRoles(userCtx, firstPageReq)
		require.NoError(t, err)
		require.NotEmpty(t, firstPageResp.NextPageToken)

		secondPageReq := &authpb.ListRequestableRolesRequest{
			PageSize:  3,
			PageToken: firstPageResp.NextPageToken,
		}
		secondPageResp, err := client.ListRequestableRoles(userCtx, secondPageReq)
		require.NoError(t, err)

		// Verify that the second page has the correct roles.
		var receivedRoles []string
		for _, role := range secondPageResp.Roles {
			receivedRoles = append(receivedRoles, role.Name)
		}
		require.Empty(t, cmp.Diff(expectedRequestableRoles[3:6], receivedRoles))

		// Verify there is no overlap in roles between the pages.
		firstPageRoleNames := make(map[string]bool)
		for _, role := range firstPageResp.Roles {
			firstPageRoleNames[role.Name] = true
		}
		for _, role := range secondPageResp.Roles {
			require.False(t, firstPageRoleNames[role.Name])
		}
	})

	t.Run("list all pages of requestable roles", func(t *testing.T) {
		limit := 3

		respRoles, err := stream.Collect(clientutils.ResourcesWithPageSize(userCtx, func(ctx context.Context, pageSize int, pageToken string) ([]*authpb.ListRequestableRolesResponse_RequestableRole, string, error) {
			req := &authpb.ListRequestableRolesRequest{
				PageSize:  int32(pageSize),
				PageToken: pageToken,
			}

			resp, err := client.ListRequestableRoles(ctx, req)
			if err != nil {
				return nil, "", err
			}

			// Verify that we got the correct page size (except for the last page).
			if resp.NextPageToken != "" {
				require.Len(t, resp.Roles, pageSize)
			}

			return resp.Roles, resp.NextPageToken, nil
		}, limit))
		require.NoError(t, err)

		// Verify that all the requestable roles were returned.
		var receivedRoles []string
		for _, role := range respRoles {
			receivedRoles = append(receivedRoles, role.Name)
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

		noPermsClient, err := tlsServer.NewClient(authtest.TestUser(noPermsUsername))
		require.NoError(t, err)
		defer noPermsClient.Close()

		req := &authpb.ListRequestableRolesRequest{}
		resp, err := noPermsClient.ListRequestableRoles(ctx, req)
		require.NoError(t, err)

		// Verify that nothing is returned.
		require.Empty(t, resp.Roles)
		require.Empty(t, resp.NextPageToken)
	})

	t.Run("list requestable roles with a search filter", func(t *testing.T) {
		req := &authpb.ListRequestableRolesRequest{
			Filter: &authpb.ListRequestableRolesRequest_Filter{
				SearchKeywords: []string{"role-99"},
			},
		}
		resp, err := client.ListRequestableRoles(userCtx, req)
		require.NoError(t, err)

		// Verify that only "role-99" was returned
		require.Len(t, resp.Roles, 1)
		require.Equal(t, "role-99", resp.Roles[0].Name)

		// There shouldn't be a nextKey
		require.Empty(t, resp.NextPageToken)
	})

}
