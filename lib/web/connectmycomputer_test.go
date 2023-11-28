// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		test := test
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
