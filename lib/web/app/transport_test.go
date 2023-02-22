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

package app

import (
	"crypto/x509/pkix"
	"fmt"
	"net/http"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func Test_transport_rewriteRedirect(t *testing.T) {
	rootCluster := "root.teleport.example.com"
	leafCluster := "leaf.teleport.example.com"

	caKey, caCert, err := tlsca.GenerateSelfSignedCA(
		pkix.Name{CommonName: rootCluster},
		[]string{rootCluster, apiutils.EncodeClusterName(rootCluster)},
		defaults.CATTL,
	)
	require.NoError(t, err)

	makeAppServer := func(cluster, appName string) types.AppServer {
		app, err := types.NewAppV3(types.Metadata{Name: appName},
			types.AppSpecV3{
				PublicAddr: fmt.Sprintf("%v.%v", appName, cluster),
				URI:        fmt.Sprintf("https://%v.internal.example.com:8888", appName),
			},
		)
		require.NoError(t, err)

		appServer, err := types.NewAppServerV3FromApp(app, cluster, "dummy")
		require.NoError(t, err)

		return appServer
	}

	clock := clockwork.NewFakeClock()

	makeTransportConfig := func(clusterName string, identity *tlsca.Identity, server types.AppServer) transportConfig {
		return transportConfig{
			clusterName: clusterName,
			identity:    identity,
			servers:     []types.AppServer{server},

			cipherSuites: utils.DefaultCipherSuites(),
			proxyClient:  &mockProxyClient{},
			accessPoint: &mockAuthClient{
				caKey:       caKey,
				caCert:      caCert,
				clusterName: clusterName,
			},
			ws: createAppSession(t, clock, caKey, caCert, clusterName, clusterName),
		}
	}

	tests := []struct {
		name            string
		transportConfig transportConfig
		respStatusCode  int
		respLocation    string
		wantLocation    string
	}{
		{
			name: "local app, no redirect",
			transportConfig: makeTransportConfig(
				rootCluster,
				&tlsca.Identity{RouteToApp: tlsca.RouteToApp{ClusterName: rootCluster}},
				makeAppServer(rootCluster, "dumper")),
			respStatusCode: 200,
			respLocation:   "",
			wantLocation:   "",
		},
		{
			name: "local app, redirect",
			transportConfig: makeTransportConfig(
				rootCluster,
				&tlsca.Identity{RouteToApp: tlsca.RouteToApp{ClusterName: rootCluster}},
				makeAppServer(rootCluster, "dumper")),
			respStatusCode: 302,
			respLocation:   "/foo/bar/baz",
			wantLocation:   "/foo/bar/baz",
		},
		{
			name: "remote app, no redirect",
			transportConfig: makeTransportConfig(
				rootCluster,
				&tlsca.Identity{RouteToApp: tlsca.RouteToApp{ClusterName: leafCluster}},
				makeAppServer(leafCluster, "dumper")),
			respStatusCode: 200,
			respLocation:   "",
			wantLocation:   "",
		},
		{
			name: "remote app, redirect to non-app addr",
			transportConfig: makeTransportConfig(
				rootCluster,
				&tlsca.Identity{RouteToApp: tlsca.RouteToApp{
					ClusterName: leafCluster,
					PublicAddr:  "dumper.leaf.teleport.example.com",
				}},
				makeAppServer(leafCluster, "dumper")),
			respStatusCode: 302,
			respLocation:   "https://google.com",
			wantLocation:   "https://google.com",
		},
		{
			name: "remote app, redirect to app public addr",
			transportConfig: makeTransportConfig(
				rootCluster,
				&tlsca.Identity{RouteToApp: tlsca.RouteToApp{
					ClusterName: leafCluster,
					PublicAddr:  "dumper.leaf.teleport.example.com",
				}},
				makeAppServer(leafCluster, "dumper")),
			respStatusCode: 302,
			respLocation:   "https://dumper.leaf.teleport.example.com:3080/admin/blah",
			wantLocation:   "/admin/blah",
		},
		{
			name: "canonicalize empty location to /",
			transportConfig: makeTransportConfig(
				rootCluster,
				&tlsca.Identity{RouteToApp: tlsca.RouteToApp{
					ClusterName: leafCluster,
					PublicAddr:  "dumper.leaf.teleport.example.com",
				}},
				makeAppServer(leafCluster, "dumper")),
			respStatusCode: 302,
			respLocation:   "https://dumper.leaf.teleport.example.com:3080",
			wantLocation:   "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			tr, err := newTransport(&tt.transportConfig)
			require.NoError(t, err)

			response := &http.Response{Header: make(http.Header)}
			response.Header.Set("Location", tt.respLocation)
			response.StatusCode = tt.respStatusCode
			err = tr.rewriteRedirect(response)
			require.NoError(t, err)

			require.Equal(t, tt.wantLocation, response.Header.Get("Location"))
		})
	}
}
