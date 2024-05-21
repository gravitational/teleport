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

package app

import (
	"context"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
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
			ws:                    createAppSession(t, clock, caKey, caCert, clusterName, clusterName),
			integrationAppHandler: &mockIntegrationAppHandler{},
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
			name: "remote app, redirect to app public addr, preserve query params",
			transportConfig: makeTransportConfig(
				rootCluster,
				&tlsca.Identity{RouteToApp: tlsca.RouteToApp{
					ClusterName: leafCluster,
					PublicAddr:  "dumper.leaf.teleport.example.com",
				}},
				makeAppServer(leafCluster, "dumper")),
			respStatusCode: 302,
			respLocation:   "https://dumper.leaf.teleport.example.com:3080/admin/blah?foo=bar&baz=bar",
			wantLocation:   "/admin/blah?foo=bar&baz=bar",
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
		{
			name: "canonicalize empty location to /, preserve query params",
			transportConfig: makeTransportConfig(
				rootCluster,
				&tlsca.Identity{RouteToApp: tlsca.RouteToApp{
					ClusterName: leafCluster,
					PublicAddr:  "dumper.leaf.teleport.example.com",
				}},
				makeAppServer(leafCluster, "dumper")),
			respStatusCode: 302,
			respLocation:   "https://dumper.leaf.teleport.example.com:3080?foo=bar&baz=bar",
			wantLocation:   "/?foo=bar&baz=bar",
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

type fakeTunnel struct {
	reversetunnelclient.Tunnel

	fakeSite *reversetunnelclient.FakeRemoteSite
	err      error
}

func (f fakeTunnel) GetSite(domainName string) (reversetunnelclient.RemoteSite, error) {
	return f.fakeSite, f.err
}

func TestTransport_DialContextNoServersAvailable(t *testing.T) {
	tp := transport{
		c: &transportConfig{
			proxyClient: fakeTunnel{
				err: trace.ConnectionProblem(errors.New(reversetunnelclient.NoApplicationTunnel), ""),
			},
			identity: &tlsca.Identity{},
			servers: []types.AppServer{
				&types.AppServerV3{Spec: types.AppServerSpecV3{App: &types.AppV3{}}},
				&types.AppServerV3{Spec: types.AppServerSpecV3{App: &types.AppV3{}}},
				&types.AppServerV3{Spec: types.AppServerSpecV3{App: &types.AppV3{}}},
			},
			log: utils.NewLoggerForTests(),
		},
	}

	ctx := context.Background()
	type dialRes struct {
		conn net.Conn
		err  error
	}

	count := len(tp.c.servers) + 1
	resC := make(chan dialRes, count)

	for i := 0; i < count; i++ {
		go func() {
			conn, err := tp.DialContext(ctx, "", "")
			resC <- dialRes{
				conn: conn,
				err:  err,
			}
		}()
	}

	for i := 0; i < count; i++ {
		res := <-resC
		require.Error(t, res.err)
		require.Nil(t, res.conn)
	}
}

type mockIntegrationAppHandler struct {
	mu   sync.Mutex
	conn net.Conn
}

func (m *mockIntegrationAppHandler) HandleConnection(conn net.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conn = conn
}

func (m *mockIntegrationAppHandler) getConnection() net.Conn {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.conn
}

func Test_transport_with_integration(t *testing.T) {
	rootCluster := "root.teleport.example.com"

	caKey, caCert, err := tlsca.GenerateSelfSignedCA(
		pkix.Name{CommonName: rootCluster},
		[]string{rootCluster, apiutils.EncodeClusterName(rootCluster)},
		defaults.CATTL,
	)
	require.NoError(t, err)

	appName := "awsconsole"
	awsApp, err := types.NewAppV3(types.Metadata{Name: appName},
		types.AppSpecV3{
			PublicAddr:  fmt.Sprintf("%v.%v", appName, rootCluster),
			URI:         fmt.Sprintf("https://%v.internal.example.com:8888", appName),
			Cloud:       types.CloudAWS,
			Integration: "my-integration",
		},
	)
	require.NoError(t, err)

	awsAppServer, err := types.NewAppServerV3FromApp(awsApp, rootCluster, "awsconsole-server")
	require.NoError(t, err)

	clock := clockwork.NewFakeClock()

	integrationAppHandler := &mockIntegrationAppHandler{}

	appSession := createAppSession(t, clock, caKey, caCert, rootCluster, rootCluster)
	tr, err := newTransport(&transportConfig{
		clusterName: rootCluster,
		identity: &tlsca.Identity{
			RouteToApp: tlsca.RouteToApp{
				ClusterName: rootCluster,
				AWSRoleARN:  "MyAWSRole",
			},
		},
		servers:      []types.AppServer{awsAppServer},
		cipherSuites: utils.DefaultCipherSuites(),
		proxyClient:  &mockProxyClient{},
		accessPoint: &mockAuthClient{
			caKey:       caKey,
			caCert:      caCert,
			clusterName: rootCluster,
		},
		ws:                    appSession,
		integrationAppHandler: integrationAppHandler,
	})
	require.NoError(t, err)

	conn, err := tr.DialContext(context.Background(), "", "")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return integrationAppHandler.getConnection() != nil
	}, 100*time.Millisecond, 10*time.Millisecond)

	message := "hello world"
	messageSize := len(message)

	go func() {
		io.WriteString(conn, message)
	}()

	bs := make([]byte, messageSize)
	_, err = io.ReadAtLeast(integrationAppHandler.getConnection(), bs, messageSize)
	require.NoError(t, err)

	require.Equal(t, message, string(bs))
}
