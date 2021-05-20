/*
Copyright 2020 Gravitational, Inc.

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

package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/require"
)

func TestUpsertServer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	const remoteAddr = "request-remote-addr"

	tests := []struct {
		desc       string
		role       teleport.Role
		reqServer  services.Server
		wantServer services.Server
		assertErr  require.ErrorAssertionFunc
	}{
		{
			desc: "node",
			reqServer: &types.ServerV2{
				Metadata: types.Metadata{Name: "test-server", Namespace: defaults.Namespace},
				Version:  services.V2,
				Kind:     services.KindNode,
			},
			role: teleport.RoleNode,
			wantServer: &types.ServerV2{
				Metadata: types.Metadata{Name: "test-server", Namespace: defaults.Namespace},
				Version:  services.V2,
				Kind:     services.KindNode,
			},
			assertErr: require.NoError,
		},
		{
			desc: "proxy",
			reqServer: &types.ServerV2{
				Metadata: types.Metadata{Name: "test-server", Namespace: defaults.Namespace},
				Version:  services.V2,
				Kind:     services.KindProxy,
			},
			role: teleport.RoleProxy,
			wantServer: &types.ServerV2{
				Metadata: types.Metadata{Name: "test-server", Namespace: defaults.Namespace},
				Version:  services.V2,
				Kind:     services.KindProxy,
			},
			assertErr: require.NoError,
		},
		{
			desc: "auth",
			reqServer: &types.ServerV2{
				Metadata: types.Metadata{Name: "test-server", Namespace: defaults.Namespace},
				Version:  services.V2,
				Kind:     services.KindAuthServer,
			},
			role: teleport.RoleAuth,
			wantServer: &types.ServerV2{
				Metadata: types.Metadata{Name: "test-server", Namespace: defaults.Namespace},
				Version:  services.V2,
				Kind:     services.KindAuthServer,
			},
			assertErr: require.NoError,
		},
		{
			desc: "unknown",
			reqServer: &types.ServerV2{
				Metadata: types.Metadata{Name: "test-server", Namespace: defaults.Namespace},
				Version:  services.V2,
				Kind:     services.KindNode,
			},
			role:      teleport.Role("unknown"),
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

			_, err = new(APIServer).upsertServer(s, tt.role, req, httprouter.Params{httprouter.Param{Key: "namespace", Value: defaults.Namespace}})
			tt.assertErr(t, err)
			if err != nil {
				return
			}

			// Fetch all servers from the backend, there should only be 1.
			var allServers []services.Server
			addServers := func(servers []services.Server, err error) {
				require.NoError(t, err)
				allServers = append(allServers, servers...)
			}
			addServers(s.GetAuthServers())
			addServers(s.GetNodes(ctx, defaults.Namespace))
			addServers(s.GetProxies())
			require.Empty(t, cmp.Diff(allServers, []services.Server{tt.wantServer}, cmpopts.IgnoreFields(types.Metadata{}, "ID")))
		})
	}
}
