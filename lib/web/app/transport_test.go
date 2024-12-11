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
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/jwt"
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
			log: utils.NewSlogLoggerForTests(),
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

func Test_transport_rewriteRequest(t *testing.T) {
	rootCluster := "root.teleport.example.com"

	caKey, caCert, err := tlsca.GenerateSelfSignedCA(
		pkix.Name{CommonName: rootCluster},
		[]string{rootCluster, apiutils.EncodeClusterName(rootCluster)},
		defaults.CATTL,
	)
	require.NoError(t, err)

	appName := "azure"
	azureApp, err := types.NewAppV3(types.Metadata{Name: appName},
		types.AppSpecV3{
			PublicAddr: fmt.Sprintf("%v.%v", appName, rootCluster),
			URI:        fmt.Sprintf("https://%v.internal.example.com:8888", appName),
			Cloud:      types.CloudAzure,
		},
	)
	require.NoError(t, err)

	azureAppServer, err := types.NewAppServerV3FromApp(azureApp, rootCluster, "azure-server")
	require.NoError(t, err)

	clock := clockwork.NewFakeClock()

	appSession := createAppSession(t, clock, caKey, caCert, rootCluster, rootCluster)
	tr, err := newTransport(&transportConfig{
		clock:       clock,
		clusterName: rootCluster,
		identity: &tlsca.Identity{
			RouteToApp: tlsca.RouteToApp{
				ClusterName:   rootCluster,
				AzureIdentity: "azure-identity",
			},
		},
		servers:      []types.AppServer{azureAppServer},
		cipherSuites: utils.DefaultCipherSuites(),
		proxyClient:  &mockProxyClient{},
		accessPoint: &mockAuthClient{
			caKey:       caKey,
			caCert:      caCert,
			clusterName: rootCluster,
		},
		ws:                    appSession,
		integrationAppHandler: &mockIntegrationAppHandler{},
	})
	require.NoError(t, err)

	t.Run("remove teleport web session cookies", func(t *testing.T) {
		request := &http.Request{Header: make(http.Header), URL: &url.URL{}}
		cookies := []*http.Cookie{
			{
				Name:  CookieName,
				Value: "teleport-cookie",
			}, {
				Name:  SubjectCookieName,
				Value: "teleport-subject-cookie",
			}, {
				Name:  "__Host-non_teleport_cookie",
				Value: "non-teleport-cookie",
			},
		}
		for _, cookie := range cookies {
			request.AddCookie(cookie)
		}

		err = tr.rewriteRequest(request)
		require.NoError(t, err)

		require.Equal(t, cookies[2:], request.Cookies())
	})

	t.Run("resign azure JWT", func(t *testing.T) {
		azureClaims := jwt.AzureTokenClaims{
			TenantID: "azure-tenant",
			Resource: "root",
		}

		clientKey, clientCertPEM := createAppKeyCertPair(t, clock, caKey, caCert, rootCluster, rootCluster)
		b, _ := pem.Decode(clientCertPEM)
		clientCert, err := x509.ParseCertificate(b.Bytes)
		require.NoError(t, err)

		wsPrivateKey, err := keys.ParsePrivateKey(appSession.GetTLSPriv())
		require.NoError(t, err)

		unknownKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
		require.NoError(t, err)

		for _, tt := range []struct {
			name          string
			jwtPrivateKey crypto.Signer
			expectErr     bool
		}{
			{
				name:          "OK signed by client key",
				jwtPrivateKey: clientKey,
			}, {
				// old clients will sign the JWT using the web session key.
				name:          "OK signed by web session key",
				jwtPrivateKey: wsPrivateKey,
			}, {
				name:          "NOK signed by unknown key",
				jwtPrivateKey: unknownKey,
				expectErr:     true,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				request := &http.Request{
					Header: make(http.Header),
					URL:    &url.URL{},
					TLS: &tls.ConnectionState{
						PeerCertificates: []*x509.Certificate{clientCert},
					},
				}

				// Sign a azure JWT for the request using the test case key.
				jwtKey, err := jwt.New(&jwt.Config{
					Clock:       tr.c.clock,
					PrivateKey:  tt.jwtPrivateKey,
					ClusterName: types.TeleportAzureMSIEndpoint,
				})
				require.NoError(t, err)

				jwtToken, err := jwtKey.SignAzureToken(azureClaims)
				require.NoError(t, err)

				request.Header.Set("Authorization", "Bearer "+jwtToken)

				// rewriteRequest should resign the jwt token with the web session
				// private key so it can be parsed by the App Service.
				err = tr.rewriteRequest(request)
				require.NoError(t, err)

				bearerToken, err := parseBearerToken(request)
				require.NoError(t, err)

				wsJWTKey, err := jwt.New(&jwt.Config{
					Clock:       tr.c.clock,
					PrivateKey:  wsPrivateKey,
					ClusterName: types.TeleportAzureMSIEndpoint,
				})
				require.NoError(t, err)

				gotClaims, err := wsJWTKey.VerifyAzureToken(bearerToken)
				if tt.expectErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					require.Equal(t, &azureClaims, gotClaims)
				}
			})
		}
	})
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
		clock:       clock,
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
