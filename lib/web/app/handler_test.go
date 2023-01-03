/*
Copyright 2021 Gravitational, Inc.

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

package app

import (
	"bytes"
	"context"
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
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
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
	const (
		stateValue  = "012ac605867e5a7d693cd6f49c7ff0fb"
		cookieValue = "5588e2be54a2834b4f152c56bafcd789f53b15477129d2ab4044e9a3c1bf0f3b"
	)

	fakeClock := clockwork.NewFakeClockAt(time.Date(2017, 05, 10, 18, 53, 0, 0, time.UTC))
	clusterName := "test-cluster"
	publicAddr := "app.example.com"
	// Generate CA TLS key and cert with the cluster and application DNS.
	key, cert, err := tlsca.GenerateSelfSignedCA(
		pkix.Name{CommonName: clusterName},
		[]string{publicAddr, apiutils.EncodeClusterName(clusterName)},
		defaults.CATTL,
	)
	require.NoError(t, err)
	appSession := createAppSession(t, fakeClock, key, cert, clusterName, publicAddr)

	tests := []struct {
		desc             string
		stateInRequest   string
		stateInCookie    string
		subjectInRequest string
		sessionError     error
		outStatusCode    int
		eventChecks      []eventCheckFn
	}{
		{
			desc:             "success",
			stateInRequest:   stateValue,
			stateInCookie:    stateValue,
			subjectInRequest: appSession.GetBearerToken(),
			outStatusCode:    http.StatusOK,
			eventChecks:      []eventCheckFn{hasAuditEventCount(0)},
		},
		{
			desc:             "missing state token in request",
			stateInRequest:   "",
			stateInCookie:    stateValue,
			subjectInRequest: appSession.GetBearerToken(),
			outStatusCode:    http.StatusForbidden,
			eventChecks:      []eventCheckFn{hasAuditEventCount(0)},
		},
		{
			desc:             "missing subject session token in request",
			stateInRequest:   stateValue,
			stateInCookie:    stateValue,
			subjectInRequest: "",
			outStatusCode:    http.StatusForbidden,
			eventChecks: []eventCheckFn{
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
						Error:   "subject session token is not set",
					},
				}),
			},
		},
		{
			desc:             "subject session token in request does not match",
			stateInRequest:   stateValue,
			stateInCookie:    stateValue,
			subjectInRequest: "foobar",
			outStatusCode:    http.StatusForbidden,
			eventChecks: []eventCheckFn{
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
						Error:   "subject session token does not match",
					},
				}),
			},
		},
		{
			desc:             "invalid session",
			stateInRequest:   stateValue,
			stateInCookie:    stateValue,
			subjectInRequest: appSession.GetBearerToken(),
			sessionError:     trace.NotFound("invalid session"),
			outStatusCode:    http.StatusForbidden,
			eventChecks:      []eventCheckFn{hasAuditEventCount(0)},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			authClient := &mockAuthClient{
				sessionError: test.sessionError,
				appSession:   appSession,
			}
			p := setup(t, fakeClock, authClient, nil)

			req, err := json.Marshal(fragmentRequest{
				StateValue:         test.stateInRequest,
				CookieValue:        cookieValue,
				SubjectCookieValue: test.subjectInRequest,
			})
			require.NoError(t, err)

			status, _ := p.makeRequest(t, "POST", "/x-teleport-auth", req, []http.Cookie{{
				Name:  AuthStateCookieName,
				Value: test.stateInCookie,
			}})
			require.Equal(t, test.outStatusCode, status)
			for _, check := range test.eventChecks {
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

	fakeClock := clockwork.NewFakeClockAt(time.Date(2017, 05, 10, 18, 53, 0, 0, time.UTC))
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
	fakeRemoteSite := reversetunnel.NewFakeRemoteSite(clusterName, authClient)
	tunnel := &reversetunnel.FakeServer{
		Sites: []reversetunnel.RemoteSite{
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

type testServer struct {
	serverURL *url.URL
}

func setup(t *testing.T, clock clockwork.FakeClock, authClient auth.ClientI, proxyClient reversetunnel.Tunnel) *testServer {
	appHandler, err := NewHandler(context.Background(), &HandlerConfig{
		Clock:        clock,
		AuthClient:   authClient,
		AccessPoint:  authClient,
		ProxyClient:  proxyClient,
		CipherSuites: utils.DefaultCipherSuites(),
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
	auth.ClientI
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

func (c *mockAuthClient) GetClusterName(_ ...services.MarshalOption) (types.ClusterName, error) {
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

func (c *mockAuthClient) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool, opts ...services.MarshalOption) (types.CertAuthority, error) {
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

// fakeRemoteListener Implements a `net.Listener` that return `net.Conn` from
// the `FakeRemoteSite`.
type fakeRemoteListener struct {
	fakeRemote *reversetunnel.FakeRemoteSite
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
func createAppSession(t *testing.T, clock clockwork.FakeClock, caKey, caCert []byte, clusterName, publicAddr string) types.WebSession {
	tlsCA, err := tlsca.FromKeys(caCert, caKey)
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

	// Generate public and private keys for the application request certificate.
	priv, pub, err := testauthority.New().GetNewKeyPairFromPool()
	require.NoError(t, err)
	cryptoPubKey, err := sshutils.CryptoPublicKey(pub)
	require.NoError(t, err)

	cert, err := tlsCA.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: cryptoPubKey,
		Subject:   subj,
		NotAfter:  clock.Now().Add(5 * time.Minute),
	})
	require.NoError(t, err)

	appSession, err := types.NewWebSession(uuid.New().String(), types.KindAppSession, types.WebSessionSpecV2{
		User:        "testuser",
		Priv:        priv,
		TLSCert:     cert,
		Expires:     clock.Now().Add(5 * time.Minute),
		BearerToken: "abc123",
	})
	require.NoError(t, err)

	return appSession
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
