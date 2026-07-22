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

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/connectmycomputer"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/web/ui"
)

func TestConnectMyComputerLoginsList(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	username := "test-user"

	tests := []struct {
		name string
		// userRoles that will be created and added to the user account.
		userRoles        []types.Role
		expectedRespCode int
		expectedLogins   []string
	}{
		{
			name:             "no Connect My Computer role",
			userRoles:        []types.Role{},
			expectedRespCode: http.StatusNotFound,
		},
		{
			name:             "with Connect My Computer role",
			userRoles:        []types.Role{makeConnectMyComputerRole(t, username)},
			expectedRespCode: http.StatusOK,
			expectedLogins:   expectedLogins,
		},
		{
			name:      "with Connect My Computer role and no access to roles",
			userRoles: []types.Role{makeConnectMyComputerRole(t, username), makeDenyAccessToRolesRole(t)},
			// The user is always able to read the roles they hold, so they should still be able to read
			// logins from connectMyComputerRole, despite having a role that denies access to read role.
			expectedRespCode: http.StatusOK,
			expectedLogins:   expectedLogins,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			env := newWebPack(t, 1)
			proxy := env.proxies[0]
			pack := proxy.authPack(t, username, test.userRoles)

			resp, err := pack.clt.Get(ctx, pack.clt.Endpoint("webapi", "connectmycomputer", "logins"), nil)
			require.Equal(t, test.expectedRespCode, resp.Code())

			if test.expectedRespCode != http.StatusOK {
				return
			}

			require.NoError(t, err)
			var listResponse ui.ConnectMyComputerLoginsListResponse
			require.NoError(t, json.Unmarshal(resp.Bytes(), &listResponse))

			require.Equal(t, test.expectedLogins, listResponse.Logins)
		})
	}
}

var expectedLogins = []string{"ssh-login-1", "ssh-login-2"}

// Instead of creating one role and then passing it to multiple parallel tests, we must create a
// fresh role for each test to avoid data races.
func makeConnectMyComputerRole(t *testing.T, username string) types.Role {
	role, err := types.NewRole(connectmycomputer.GetRoleNameForUser(username), types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: expectedLogins,
		}})
	require.NoError(t, err)
	return role
}

func makeDenyAccessToRolesRole(t *testing.T) types.Role {
	role, err := types.NewRole("no-roles", types.RoleSpecV6{
		Deny: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindRole, services.RW()),
			},
		},
	})
	require.NoError(t, err)
	return role
}
