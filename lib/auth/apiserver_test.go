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

package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestUpsertServer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	const remoteAddr = "request-remote-addr"

	tests := []struct {
		desc       string
		role       types.SystemRole
		reqServer  types.Server
		wantServer types.Server
		assertErr  require.ErrorAssertionFunc
	}{
		{
			desc: "node",
			reqServer: &types.ServerV2{
				Metadata: types.Metadata{Name: "test-server", Namespace: apidefaults.Namespace},
				Version:  types.V2,
				Kind:     types.KindNode,
			},
			role: types.RoleNode,
			wantServer: &types.ServerV2{
				Metadata: types.Metadata{Name: "test-server", Namespace: apidefaults.Namespace},
				Version:  types.V2,
				Kind:     types.KindNode,
			},
			assertErr: require.NoError,
		},
		{
			desc: "proxy",
			reqServer: &types.ServerV2{
				Metadata: types.Metadata{Name: "test-server", Namespace: apidefaults.Namespace},
				Version:  types.V2,
				Kind:     types.KindProxy,
			},
			role: types.RoleProxy,
			wantServer: &types.ServerV2{
				Metadata: types.Metadata{Name: "test-server", Namespace: apidefaults.Namespace},
				Version:  types.V2,
				Kind:     types.KindProxy,
			},
			assertErr: require.NoError,
		},
		{
			desc: "auth",
			reqServer: &types.ServerV2{
				Metadata: types.Metadata{Name: "test-server", Namespace: apidefaults.Namespace},
				Version:  types.V2,
				Kind:     types.KindAuthServer,
			},
			role: types.RoleAuth,
			wantServer: &types.ServerV2{
				Metadata: types.Metadata{Name: "test-server", Namespace: apidefaults.Namespace},
				Version:  types.V2,
				Kind:     types.KindAuthServer,
			},
			assertErr: require.NoError,
		},
		{
			desc: "unknown",
			reqServer: &types.ServerV2{
				Metadata: types.Metadata{Name: "test-server", Namespace: apidefaults.Namespace},
				Version:  types.V2,
				Kind:     types.KindNode,
			},
			role:      types.SystemRole("unknown"),
			assertErr: require.Error,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()
			// Set up backend to upsert servers into.
			s := newTestServices(t)

			// Create a fake HTTP request.
			inSrv, err := services.MarshalServer(tt.reqServer)
			require.NoError(t, err)
			body, err := json.Marshal(upsertServerRawReq{Server: inSrv})
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "http://localhost", bytes.NewReader(body))
			req.RemoteAddr = remoteAddr
			req.Header.Add("Content-Type", "application/json")

			_, err = new(APIServer).upsertServer(s, tt.role, req, httprouter.Params{httprouter.Param{Key: "namespace", Value: apidefaults.Namespace}})
			tt.assertErr(t, err)
			if err != nil {
				return
			}

			// Fetch all servers from the backend, there should only be 1.
			var allServers []types.Server
			addServers := func(servers []types.Server, err error) {
				require.NoError(t, err)
				allServers = append(allServers, servers...)
			}
			addServers(s.GetAuthServers())
			addServers(s.GetNodes(ctx, apidefaults.Namespace))
			addServers(s.GetProxies())
			require.Empty(t, cmp.Diff(allServers, []types.Server{tt.wantServer}, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
		})
	}
}

func TestUpsertUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testSrv := newTestTLSServer(t)

	role, err := types.NewRole("role-that-exists", types.RoleSpecV6{})
	require.NoError(t, err)
	role, err = testSrv.Auth().CreateRole(ctx, role)
	require.NoError(t, err)

	c, err := testSrv.NewClient(TestAdmin())
	require.NoError(t, err)
	defer c.Close()

	tests := []struct {
		desc      string
		role      string
		assertErr require.ErrorAssertionFunc
	}{
		{
			desc:      "existing role",
			role:      role.GetName(),
			assertErr: require.NoError,
		}, {
			desc: "role that doesn't exist",
			role: "some-other-role",
			assertErr: func(t require.TestingT, err error, args ...interface{}) {
				require.True(t, trace.IsNotFound(err))
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			inUsr, err := services.MarshalUser(&types.UserV2{
				Metadata: types.Metadata{Name: fmt.Sprintf("test-user-%s", tt.role), Namespace: apidefaults.Namespace},
				Version:  types.V2,
				Kind:     types.KindUser,
				Spec: types.UserSpecV2{
					Roles: []string{tt.role},
				},
			})
			require.NoError(t, err)

			_, err = c.HTTPClient.PostJSON(ctx, c.Endpoint("users"), &upsertUserRawReq{
				User: inUsr,
			})
			tt.assertErr(t, err)
		})
	}
}
