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
	"bytes"
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

type eventCheckFn func(t *testing.T, events []apievents.AuditEvent)

func hasAuditEvent(idx int, want apievents.AuditEvent) eventCheckFn {
	return func(t *testing.T, events []apievents.AuditEvent) {
		t.Helper()
		require.Greater(t, len(events), idx)
		require.Empty(t, cmp.Diff(want, events[idx],
			cmpopts.IgnoreFields(apievents.AuthAttempt{}, "ConnectionMetadata")))
	}
}

func hasAuditEventCount(want int) eventCheckFn {
	return func(t *testing.T, events []apievents.AuditEvent) {
		t.Helper()
		require.Len(t, events, want)
	}
}

// TestAuthPOST tests the handler of POST /x-teleport-auth.
func TestAuthPOST(t *testing.T) {
	secretToken := "012ac605867e5a7d693cd6f49c7ff0fb"
	cookieID := "cookie-name"
	stateValue := fmt.Sprintf("%s_%s", secretToken, cookieID)
	appCookieValue := "5588e2be54a2834b4f152c56bafcd789f53b15477129d2ab4044e9a3c1bf0f3b"

	fakeClock := clockwork.NewFakeClock()
	clusterName := "test-cluster"
	publicAddr := "app.example.com"
	// Generate CA TLS key and cert with the cluster and application DNS.
	key, cert, err := tlsca.GenerateSelfSignedCA(
		pkix.Name{CommonName: clusterName},
		[]string{publicAddr, apiutils.EncodeClusterName(clusterName)},
		defaults.CATTL,
	)
	require.NoError(t, err)

	tests := []struct {
		desc            string
		sessionError    error
		outStatusCode   int
		makeRequestBody func(types.WebSession) fragmentRequest
		getEventChecks  func(types.WebSession) []eventCheckFn
	}{
		{
			desc: "success",
			makeRequestBody: func(appSession types.WebSession) fragmentRequest {
				return fragmentRequest{
					StateValue:         stateValue,
					CookieValue:        appCookieValue,
					SubjectCookieValue: appSession.GetBearerToken(),
				}
			},
			outStatusCode: http.StatusOK,
			getEventChecks: func(types.WebSession) []eventCheckFn {
				return []eventCheckFn{hasAuditEventCount(0)}
			},
		},
		{
			desc: "missing state token in request",
			makeRequestBody: func(appSession types.WebSession) fragmentRequest {
				return fragmentRequest{
					StateValue:         "",
					CookieValue:        appCookieValue,
					SubjectCookieValue: appSession.GetBearerToken(),
				}
			},
			outStatusCode: http.StatusForbidden,
			getEventChecks: func(types.WebSession) []eventCheckFn {
				return []eventCheckFn{
					hasAuditEventCount(1),
					hasAuditEvent(0, &apievents.AuthAttempt{
						Metadata: apievents.Metadata{
							Type: events.AuthAttemptEvent,
							Code: events.AuthAttemptFailureCode,
						},
						UserMetadata: apievents.UserMetadata{
							User: "unknown",
						},
						Status: apievents.Status{
							Success: false,
							Error:   "Failed app access authentication: state token was not in the expected format",
						},
					}),
				}
			},
		},
		{
			desc: "missing subject session token in request",
			makeRequestBody: func(ws types.WebSession) fragmentRequest {
				return fragmentRequest{
					StateValue:         stateValue,
					CookieValue:        appCookieValue,
					SubjectCookieValue: "",
				}
			},
			outStatusCode: http.StatusForbidden,
			getEventChecks: func(appSession types.WebSession) []eventCheckFn {
				return []eventCheckFn{
					hasAuditEventCount(1),
					hasAuditEvent(0, &apievents.AuthAttempt{
						Metadata: apievents.Metadata{
							Type: events.AuthAttemptEvent,
							Code: events.AuthAttemptFailureCode,
						},
						UserMetadata: apievents.UserMetadata{
							User:  "unknown",
							Login: "testuser",
						},
						Status: apievents.Status{
							Success: false,
							Error:   "Failed app access authentication: subject session token is not set",
						},
					}),
				}
			},
		},
		{
			desc: "subject session token in request does not match",
			makeRequestBody: func(ws types.WebSession) fragmentRequest {
				return fragmentRequest{
					StateValue:         stateValue,
					CookieValue:        appCookieValue,
					SubjectCookieValue: "foobar",
				}
			},
			outStatusCode: http.StatusForbidden,
			getEventChecks: func(appSession types.WebSession) []eventCheckFn {
				return []eventCheckFn{
					hasAuditEventCount(1),
					hasAuditEvent(0, &apievents.AuthAttempt{
						Metadata: apievents.Metadata{
							Type: events.AuthAttemptEvent,
							Code: events.AuthAttemptFailureCode,
						},
						UserMetadata: apievents.UserMetadata{
							Login: appSession.GetUser(),
							User:  "unknown",
						},
						Status: apievents.Status{
							Success: false,
							Error:   "Failed app access authentication: subject session token does not match",
						},
					}),
				}
			},
		},
		{
			desc: "invalid session",
			makeRequestBody: func(appSession types.WebSession) fragmentRequest {
				return fragmentRequest{
					StateValue:         stateValue,
					CookieValue:        appCookieValue,
					SubjectCookieValue: appSession.GetBearerToken(),
				}
			},
			sessionError:  trace.NotFound("invalid session"),
			outStatusCode: http.StatusForbidden,
			getEventChecks: func(types.WebSession) []eventCheckFn {
				return []eventCheckFn{hasAuditEventCount(0)}
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			appSession := createAppSession(t, fakeClock, key, cert, clusterName, publicAddr)
			authClient := &mockAuthClient{
				sessionError: test.sessionError,
				appSession:   appSession,
			}
			p := setup(t, fakeClock, authClient, nil)

			reqBody := test.makeRequestBody(appSession)
			req, err := json.Marshal(reqBody)
			require.NoError(t, err)

			status, _ := p.makeRequest(t, "POST", "/x-teleport-auth", req, []http.Cookie{{
				Name:  fmt.Sprintf("%s_%s", AuthStateCookieName, cookieID),
				Value: secretToken,
			}})
			require.Equal(t, status, test.outStatusCode)
			for _, check := range test.getEventChecks(appSession) {
				check(t, authClient.emittedEvents)
			}
		})
	}
}

func TestHasName(t *testing.T) {
	for _, test := range []struct {
		desc        string
		addrs       []string
		reqHost     string
		reqURL      string
		expectedURL string
		hasName     bool
	}{
		{
			desc:        "NOK - invalid host",
			addrs:       []string{"proxy.com"},
			reqURL:      "badurl.com",
			expectedURL: "",
			hasName:     false,
		},
		{
			desc:        "OK - adds path",
			addrs:       []string{"proxy.com"},
			reqURL:      "https://app1.proxy.com/foo",
			expectedURL: "https://proxy.com/web/launch/app1.proxy.com?path=%2Ffoo",
			hasName:     true,
		},
		{
			desc:        "OK - adds paths with ampersands",
			addrs:       []string{"proxy.com"},
			reqURL:      "https://app1.proxy.com/foo/this&/that",
			expectedURL: "https://proxy.com/web/launch/app1.proxy.com?path=%2Ffoo%2Fthis%26%2Fthat",
			hasName:     true,
		},
		{
			desc:        "OK - adds root path",
			addrs:       []string{"proxy.com"},
			reqURL:      "https://app1.proxy.com/",
			expectedURL: "https://proxy.com/web/launch/app1.proxy.com?path=%2F",
			hasName:     true,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, test.reqURL, nil)
			require.NoError(t, err)

			addrs := utils.MustParseAddrList(test.addrs...)
			u, ok := HasName(req, addrs)
			require.Equal(t, test.expectedURL, u)
			require.Equal(t, test.hasName, ok)
		})
	}
}

func TestMatchApplicationServers(t *testing.T) {
	clusterName := "test-cluster"
	publicAddr := "app.example.com"

	// Generate CA TLS key and cert with the cluster and application DNS.
	key, cert, err := tlsca.GenerateSelfSignedCA(
		pkix.Name{CommonName: clusterName},
		[]string{publicAddr, apiutils.EncodeClusterName(clusterName)},
		defaults.CATTL,
	)
	require.NoError(t, err)

	fakeClock := clockwork.NewFakeClock()
	authClient := &mockAuthClient{
		clusterName: clusterName,
		appSession:  createAppSession(t, fakeClock, key, cert, clusterName, publicAddr),
		// Three app servers with same public addr from our session, and three
		// that won't match.
		appServers: []types.AppServer{
			createAppServer(t, publicAddr),
			createAppServer(t, publicAddr),
			createAppServer(t, publicAddr),
			createAppServer(t, "random.example.com"),
			createAppServer(t, "random2.example.com"),
			createAppServer(t, "random3.example.com"),
		},
		caKey:  key,
		caCert: cert,
	}

	// Create a fake remote site and tunnel.
	fakeRemoteSite := reversetunnelclient.NewFakeRemoteSite(clusterName, authClient)
	tunnel := &reversetunnelclient.FakeServer{
		Sites: []reversetunnelclient.RemoteSite{
			fakeRemoteSite,
		},
	}

	// Create a httptest server to serve the application requests. It must serve
	// TLS content with the generated certificate.
	tlsCert, err := tls.X509KeyPair(cert, key)
	require.NoError(t, err)
	expectedContent := "Hello from application"
	server := &httptest.Server{
		TLS: &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
		},
		Listener: &fakeRemoteListener{fakeRemoteSite},
		Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, expectedContent)
		})},
	}
	server.StartTLS()

	// Teardown the remote site and the httptest server.
	t.Cleanup(func() {
		require.NoError(t, fakeRemoteSite.Close())
		server.Close()
	})

	p := setup(t, fakeClock, authClient, tunnel)
	status, content := p.makeRequest(t, "GET", "/", []byte{}, []http.Cookie{
		{
			Name:  CookieName,
			Value: "abc",
		},
		{
			Name:  SubjectCookieName,
			Value: authClient.appSession.GetBearerToken(),
		},
	})

	require.Equal(t, http.StatusOK, status)
	// Remote site should receive only 4 connection requests: 3 from the
	// MatchHealthy and 1 from the transport.
	require.Equal(t, int64(4), fakeRemoteSite.DialCount())
	// Guarantee the request was returned by the httptest server.
	require.Equal(t, expectedContent, content)
}

func TestHealthCheckAppServer(t *testing.T) {
	ctx := context.Background()
	clusterName := "test-cluster"
	publicAddr := "valid.example.com"

	key, cert, err := tlsca.GenerateSelfSignedCA(
		pkix.Name{CommonName: clusterName},
		[]string{publicAddr, apiutils.EncodeClusterName(clusterName)},
		defaults.CATTL,
	)
	require.NoError(t, err)

	for _, tc := range []struct {
		desc                string
		publicAddr          string
		appServersFunc      func(t *testing.T, remoteSite *reversetunnelclient.FakeRemoteSite) []types.AppServer
		expectedTunnelCalls int
		expectErr           require.ErrorAssertionFunc
	}{
		{
			desc:       "match and online services",
			publicAddr: "valid.example.com",
			appServersFunc: func(t *testing.T, _ *reversetunnelclient.FakeRemoteSite) []types.AppServer {
				return []types.AppServer{createAppServer(t, "valid.example.com")}
			},
			expectedTunnelCalls: 1,
			expectErr:           require.NoError,
		},
		{
			desc:       "match and but no online services",
			publicAddr: "valid.example.com",
			appServersFunc: func(t *testing.T, tunnel *reversetunnelclient.FakeRemoteSite) []types.AppServer {
				appServer := createAppServer(t, "valid.example.com")
				tunnel.OfflineTunnels = map[string]struct{}{
					fmt.Sprintf("%s.%s", appServer.GetHostID(), clusterName): {},
				}
				return []types.AppServer{appServer}
			},
			expectedTunnelCalls: 1,
			expectErr:           require.Error,
		},
		{
			desc:       "no match",
			publicAddr: "valid.example.com",
			appServersFunc: func(t *testing.T, tunnel *reversetunnelclient.FakeRemoteSite) []types.AppServer {
				return []types.AppServer{}
			},
			expectedTunnelCalls: 0,
			expectErr:           require.Error,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fakeClock := clockwork.NewFakeClock()
			appSession := createAppSession(t, fakeClock, key, cert, clusterName, publicAddr)
			authClient := &mockAuthClient{
				clusterName: clusterName,
				appSession:  appSession,
				caKey:       key,
				caCert:      cert,
			}

			fakeRemoteSite := reversetunnelclient.NewFakeRemoteSite(clusterName, authClient)
			authClient.appServers = tc.appServersFunc(t, fakeRemoteSite)

			// Create a httptest server to serve the application requests. It must serve
			// TLS content with the generated certificate.
			tlsCert, err := tls.X509KeyPair(cert, key)
			require.NoError(t, err)
			server := &httptest.Server{
				TLS: &tls.Config{
					Certificates: []tls.Certificate{tlsCert},
				},
				Listener: &fakeRemoteListener{fakeRemoteSite},
				Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					fmt.Fprint(w, "Hello application")
				})},
			}
			server.StartTLS()

			tunnel := &reversetunnelclient.FakeServer{
				Sites: []reversetunnelclient.RemoteSite{fakeRemoteSite},
			}

			appHandler, err := NewHandler(ctx, &HandlerConfig{
				Clock:                 fakeClock,
				AuthClient:            authClient,
				AccessPoint:           authClient,
				ProxyClient:           tunnel,
				CipherSuites:          utils.DefaultCipherSuites(),
				IntegrationAppHandler: &mockIntegrationAppHandler{},
			})
			require.NoError(t, err)

			err = appHandler.HealthCheckAppServer(ctx, tc.publicAddr, clusterName)
			tc.expectErr(t, err)
			require.Equal(t, int64(tc.expectedTunnelCalls), fakeRemoteSite.DialCount())
		})
	}
}

type testServer struct {
	serverURL *url.URL
}

func setup(t *testing.T, clock *clockwork.FakeClock, authClient authclient.ClientI, proxyClient reversetunnelclient.Tunnel) *testServer {
	appHandler, err := NewHandler(context.Background(), &HandlerConfig{
		Clock:                 clock,
		AuthClient:            authClient,
		AccessPoint:           authClient,
		ProxyClient:           proxyClient,
		CipherSuites:          utils.DefaultCipherSuites(),
		IntegrationAppHandler: &mockIntegrationAppHandler{},
	})
	require.NoError(t, err)

	server := httptest.NewUnstartedServer(appHandler)
	server.StartTLS()

	url, err := url.Parse(server.URL)
	require.NoError(t, err)

	return &testServer{
		serverURL: url,
	}
}

func (p *testServer) makeRequest(t *testing.T, method, endpoint string, reqBody []byte, cookies []http.Cookie) (int, string) {
	u := url.URL{
		Scheme: p.serverURL.Scheme,
		Host:   p.serverURL.Host,
		Path:   endpoint,
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewBuffer(reqBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// Attach the cookie.
	for _, c := range cookies {
		req.AddCookie(&c)
	}

	// Issue request.
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	require.NoError(t, err)

	content, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.NoError(t, resp.Body.Close())
	return resp.StatusCode, string(content)
}

type mockAuthClient struct {
	authclient.ClientI
	clusterName   string
	appSession    types.WebSession
	sessionError  error
	appServers    []types.AppServer
	caKey         []byte
	caCert        []byte
	emittedEvents []apievents.AuditEvent
	mtx           sync.Mutex
}

type mockClusterName struct {
	types.ClusterName
	name string
}

func (c *mockAuthClient) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	c.emittedEvents = append(c.emittedEvents, event)
	return nil
}

func (c *mockAuthClient) DeleteAppSession(ctx context.Context, r types.DeleteAppSessionRequest) error {
	return nil
}

func (c *mockAuthClient) GetClusterName(_ context.Context) (types.ClusterName, error) {
	return mockClusterName{name: c.clusterName}, nil
}

func (n mockClusterName) GetClusterName() string {
	if n.name != "" {
		return n.name
	}

	return "local-cluster"
}

func (c *mockAuthClient) GetAppSession(context.Context, types.GetAppSessionRequest) (types.WebSession, error) {
	return c.appSession, c.sessionError
}

func (c *mockAuthClient) GetApplicationServers(_ context.Context, _ string) ([]types.AppServer, error) {
	return c.appServers, nil
}

func (c *mockAuthClient) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: c.clusterName,
		ActiveKeys: types.CAKeySet{
			TLS: []*types.TLSKeyPair{{
				Cert: c.caCert,
				Key:  c.caKey,
			}},
		},
	})
	if err != nil {
		return nil, err
	}

	return ca, nil
}

func (c *mockAuthClient) NewWatcher(context.Context, types.Watch) (types.Watcher, error) {
	return nil, trace.NotImplemented("")
}

func (c *mockAuthClient) GetProxies() ([]types.Server, error) {
	return []types.Server{}, nil
}

// fakeRemoteListener Implements a `net.Listener` that return `net.Conn` from
// the `FakeRemoteSite`.
type fakeRemoteListener struct {
	fakeRemote *reversetunnelclient.FakeRemoteSite
}

func (r *fakeRemoteListener) Accept() (net.Conn, error) {
	conn, ok := <-r.fakeRemote.ProxyConn()
	if !ok {
		return nil, fmt.Errorf("remote closed")
	}

	return conn, nil
}

func (r *fakeRemoteListener) Close() error {
	return nil
}

func (r *fakeRemoteListener) Addr() net.Addr {
	return &net.IPAddr{}
}

// createAppSession generates a WebSession for an application.
func createAppSession(t *testing.T, clock *clockwork.FakeClock, caKey, caCert []byte, clusterName, publicAddr string) types.WebSession {
	key, cert := createAppKeyCertPair(t, clock, caKey, caCert, clusterName, publicAddr)
	keyPEM, err := keys.MarshalPrivateKey(key)
	require.NoError(t, err)
	appSession, err := types.NewWebSession(uuid.New().String(), types.KindAppSession, types.WebSessionSpecV2{
		User:        "testuser",
		TLSPriv:     keyPEM,
		TLSCert:     cert,
		Expires:     clock.Now().Add(5 * time.Minute),
		BearerToken: "abc123",
	})
	require.NoError(t, err)

	return appSession
}

// createAppKeyCertPair creates and a client key and signed app cert for the client key
func createAppKeyCertPair(t *testing.T, clock *clockwork.FakeClock, caKey, caCert []byte, clusterName, publicAddr string) (crypto.Signer, []byte) {
	tlsCA, err := tlsca.FromKeys(caCert, caKey)
	require.NoError(t, err)

	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	// Generate the identity with a `RouteToApp` option.
	subj, err := (&tlsca.Identity{
		Username: "testuser",
		Groups:   []string{"access"},
		RouteToApp: tlsca.RouteToApp{
			PublicAddr:  publicAddr,
			ClusterName: clusterName,
			Name:        "testapp",
		},
	}).Subject()
	require.NoError(t, err)

	cert, err := tlsCA.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   subj,
		NotAfter:  clock.Now().Add(5 * time.Minute),
	})
	require.NoError(t, err)

	return privateKey, cert
}

func createAppServer(t *testing.T, publicAddr string) types.AppServer {
	appName := uuid.New().String()
	appServer, err := types.NewAppServerV3(
		types.Metadata{Name: appName},
		types.AppServerSpecV3{
			HostID: uuid.New().String(),
			App: &types.AppV3{
				Metadata: types.Metadata{Name: appName},
				Spec: types.AppSpecV3{
					URI:        "localhost",
					PublicAddr: publicAddr,
				},
			},
		},
	)
	require.NoError(t, err)
	return appServer
}

func TestMakeAppRedirectURL(t *testing.T) {
	for _, test := range []struct {
		name             string
		reqURL           string
		expectedURL      string
		launderURLParams launcherURLParams
	}{
		// with launcherURLParams empty (will be empty if user did not launch app from our web UI)
		{
			name:        "OK - no path",
			reqURL:      "https://grafana.localhost",
			expectedURL: "https://proxy.com/web/launch/grafana.localhost?path=",
		},
		{
			name:        "OK - add root path",
			reqURL:      "https://grafana.localhost/",
			expectedURL: "https://proxy.com/web/launch/grafana.localhost?path=%2F",
		},
		{
			name:        "OK - add multi path",
			reqURL:      "https://grafana.localhost/foo/bar",
			expectedURL: "https://proxy.com/web/launch/grafana.localhost?path=%2Ffoo%2Fbar",
		},
		{
			name:        "OK - add paths with ampersands",
			reqURL:      "https://grafana.localhost/foo/this&/that",
			expectedURL: "https://proxy.com/web/launch/grafana.localhost?path=%2Ffoo%2Fthis%26%2Fthat",
		},
		{
			name:        "OK - add only query",
			reqURL:      "https://grafana.localhost?foo=bar",
			expectedURL: "https://proxy.com/web/launch/grafana.localhost?path=&query=foo%3Dbar",
		},
		{
			name:        "OK - add query with same keys used to store the original path and query",
			reqURL:      "https://grafana.localhost?foo=bar&query=test1&path=test",
			expectedURL: "https://proxy.com/web/launch/grafana.localhost?path=&query=foo%3Dbar%26query%3Dtest1%26path%3Dtest",
		},
		{
			name:        "OK - adds query with root path",
			reqURL:      "https://grafana.localhost/?foo=bar&baz=qux&fruit=apple",
			expectedURL: "https://proxy.com/web/launch/grafana.localhost?path=%2F&query=foo%3Dbar%26baz%3Dqux%26fruit%3Dapple",
		},
		{
			name:        "OK - real grafana query example (encoded spaces)",
			reqURL:      "https://grafana.localhost/alerting/list?search=state:inactive%20type:alerting%20health:nodata",
			expectedURL: "https://proxy.com/web/launch/grafana.localhost?path=%2Falerting%2Flist&query=search%3Dstate%3Ainactive%2520type%3Aalerting%2520health%3Anodata",
		},
		{
			name:        "OK - query with non-encoded spaces",
			reqURL:      "https://grafana.localhost/alerting /list?search=state:inactive type:alerting health:nodata",
			expectedURL: "https://proxy.com/web/launch/grafana.localhost?path=%2Falerting+%2Flist&query=search%3Dstate%3Ainactive+type%3Aalerting+health%3Anodata",
		},

		// with launcherURLParams (defined if user used the "launcher" button from our web UI)
		{
			name: "OK - with clusterId and publicAddr",
			launderURLParams: launcherURLParams{
				stateToken:  "abc123",
				clusterName: "im-a-cluster-name",
				publicAddr:  "grafana.localhost",
			},
			expectedURL: "https://proxy.com/web/launch/grafana.localhost/im-a-cluster-name/grafana.localhost?path=&required-apps=&state=abc123",
		},
		{
			name: "OK - with clusterId, publicAddr, and arn",
			launderURLParams: launcherURLParams{
				stateToken:  "abc123",
				clusterName: "im-a-cluster-name",
				publicAddr:  "grafana.localhost",
				arn:         "arn:aws:iam::123456789012:role%2Frole-name",
			},
			expectedURL: "https://proxy.com/web/launch/grafana.localhost/im-a-cluster-name/grafana.localhost/arn:aws:iam::123456789012:role%252Frole-name?path=&required-apps=&state=abc123",
		},
		{
			name: "OK - with clusterId, publicAddr, arn and path",
			launderURLParams: launcherURLParams{
				stateToken:  "abc123",
				clusterName: "im-a-cluster-name",
				publicAddr:  "grafana.localhost",
				arn:         "arn:aws:iam::123456789012:role%2Frole-name",
				path:        "/foo/bar?qux=qex",
			},
			expectedURL: "https://proxy.com/web/launch/grafana.localhost/im-a-cluster-name/grafana.localhost/arn:aws:iam::123456789012:role%252Frole-name?path=%2Ffoo%2Fbar%3Fqux%3Dqex&required-apps=&state=abc123",
		},
		{
			name: "OK - with clusterId, publicAddr, arn, path, and required-apps",
			launderURLParams: launcherURLParams{
				stateToken:       "abc123",
				clusterName:      "im-a-cluster-name",
				publicAddr:       "grafana.localhost",
				arn:              "arn:aws:iam::123456789012:role%2Frole-name",
				path:             "/foo/bar?qux=qex",
				requiredAppFQDNs: "api.example.com,grafana.localhost",
			},
			expectedURL: "https://proxy.com/web/launch/grafana.localhost/im-a-cluster-name/grafana.localhost/arn:aws:iam::123456789012:role%252Frole-name?path=%2Ffoo%2Fbar%3Fqux%3Dqex&required-apps=api.example.com%2Cgrafana.localhost&state=abc123",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, test.reqURL, nil)
			require.NoError(t, err)

			urlStr := makeAppRedirectURL(req, "proxy.com", "grafana.localhost", test.launderURLParams)
			require.Equal(t, test.expectedURL, urlStr)
		})
	}
}
