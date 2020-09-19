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

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/app"
	"github.com/gravitational/teleport/lib/srv/regular"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
)

// TestAuthenticate verifies only valid cookies return authenticated sessions.
func TestAuthenticate(t *testing.T) {
	pack, done := setup(t)
	defer done()

	// Create application handler that runs within the web handlers.
	authClient, err := pack.s.tlsServer.NewClient(auth.TestBuiltin(teleport.RoleWeb))
	assert.Nil(t, err)
	handler, err := NewHandler(&HandlerConfig{
		AuthClient:  authClient,
		ProxyClient: pack.p.tunnel,
	})
	assert.Nil(t, err)

	// Create a web UI session.
	webSession, err := pack.s.client.CreateWebSession(pack.u.user.GetName())
	assert.Nil(t, err)

	// Use the Web UI session to ask for a application specific session.
	appSession, err := pack.u.client.CreateAppSession(context.Background(), services.CreateAppSessionRequest{
		AppName:     pack.a.application.Name,
		ClusterName: pack.s.tlsServer.ClusterName(),
		SessionID:   webSession.GetName(),
		BearerToken: webSession.GetBearerToken(),
	})
	assert.Nil(t, err)

	// Create a few cookie values to test.
	validCookieValue, err := encodeCookie(&Cookie{
		Username:   pack.u.user.GetName(),
		ParentHash: appSession.GetParentHash(),
		SessionID:  appSession.GetName(),
	})
	assert.Nil(t, err)
	invalidCookieValue, err := encodeCookie(&Cookie{
		Username:   pack.u.user.GetName(),
		ParentHash: appSession.GetParentHash(),
		SessionID:  "invalid-session-id",
	})
	assert.Nil(t, err)

	var tests = []struct {
		desc     string
		inCookie *http.Cookie
		outError bool
	}{
		{
			desc:     "Missing cookie.",
			inCookie: &http.Cookie{},
			outError: true,
		},
		{
			desc: "Invalid cookie.",
			inCookie: &http.Cookie{
				Name:  cookieName,
				Value: invalidCookieValue,
			},
			outError: true,
		},
		{
			desc: "Valid cookie.",
			inCookie: &http.Cookie{
				Name:  cookieName,
				Value: validCookieValue,
			},
			outError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Create a request and add in the cookie.
			r, err := http.NewRequest(http.MethodGet, "/", nil)
			assert.Nil(t, err)
			r.AddCookie(tt.inCookie)

			// Attempt to authenticate the session.
			_, err = handler.authenticate(r)
			assert.Equal(t, tt.outError, err != nil)
		})
	}

	// TODO(russjones): Delete the application session, then delete the UI
	// session and make sure access is fully revoked.
}

// TestForward verifies the request is updated (jwt header added, session
// cookies removed) and forwarded to the target application. When the
// underlying tunnel connection is closed, forwarding should fail as well.
func TestForward(t *testing.T) {
	pack, done := setup(t)
	defer done()

	// Create application handler that runs within the web handlers.
	authClient, err := pack.s.tlsServer.NewClient(auth.TestBuiltin(teleport.RoleWeb))
	assert.Nil(t, err)
	handler, err := NewHandler(&HandlerConfig{
		AuthClient:  authClient,
		ProxyClient: pack.p.tunnel,
	})
	assert.Nil(t, err)

	// Create a web UI session.
	webSession, err := pack.s.client.CreateWebSession(pack.u.user.GetName())
	assert.Nil(t, err)

	// Use the Web UI session to ask for a application specific session.
	appSession, err := pack.u.client.CreateAppSession(context.Background(), services.CreateAppSessionRequest{
		AppName:     pack.a.application.Name,
		ClusterName: pack.s.tlsServer.ClusterName(),
		SessionID:   webSession.GetName(),
		BearerToken: webSession.GetBearerToken(),
	})
	assert.Nil(t, err)

	// Create a session cache and create a session.
	cache, err := newSessionCache(&sessionCacheConfig{
		AuthClient:  authClient,
		ProxyClient: pack.p.tunnel,
	})
	assert.Nil(t, err)
	session, err := cache.newSession(context.Background(), "not-required", appSession)
	assert.Nil(t, err)

	// Create a request to the requested target.
	r, err := http.NewRequest(http.MethodGet, pack.a.application.PublicAddr, nil)
	assert.Nil(t, err)

	// Issue the request, it should succeed.
	w := httptest.NewRecorder()
	handler.forward(w, r, session)
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)

	// Check that the output contains the expected jwt.
	resp := w.Result()
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(t, err)
	assert.Equal(t, string(body), session.jwt)

	// Close the underlying connection.
	err = session.conn.Close()
	assert.Nil(t, err)

	// Issue the request once again, it should fail this time.
	w = httptest.NewRecorder()
	handler.forward(w, r, session)
	assert.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)

	// TODO(russjones): Check that the authentication cookie is removed.
	// TODO(russjones): Update "handler.forward" to return an error. Here only
	// check that an error is returned in session_test.go check that it also
	// removes the session.
}

// TestFragment validates fragment validation works, that a valid session cookie will be retu
func TestFragment(t *testing.T) {
	pack, done := setup(t)
	defer done()

	// Create application handler that runs within the web handlers.
	authClient, err := pack.s.tlsServer.NewClient(auth.TestBuiltin(teleport.RoleWeb))
	assert.Nil(t, err)
	handler, err := NewHandler(&HandlerConfig{
		AuthClient:  authClient,
		ProxyClient: pack.p.tunnel,
	})
	assert.Nil(t, err)

	// Create a web UI session.
	webSession, err := pack.s.client.CreateWebSession(pack.u.user.GetName())
	assert.Nil(t, err)

	// Use the Web UI session to ask for a application specific session.
	appSession, err := pack.u.client.CreateAppSession(context.Background(), services.CreateAppSessionRequest{
		AppName:     pack.a.application.Name,
		ClusterName: pack.s.tlsServer.ClusterName(),
		SessionID:   webSession.GetName(),
		BearerToken: webSession.GetBearerToken(),
	})
	assert.Nil(t, err)

	// Create a few cookie values to test.
	validCookieValue, err := encodeCookie(&Cookie{
		Username:   pack.u.user.GetName(),
		ParentHash: appSession.GetParentHash(),
		SessionID:  appSession.GetName(),
	})
	assert.Nil(t, err)
	invalidCookieValue, err := encodeCookie(&Cookie{
		Username:   pack.u.user.GetName(),
		ParentHash: appSession.GetParentHash(),
		SessionID:  "invalid-session-id",
	})
	assert.Nil(t, err)

	var tests = []struct {
		desc          string
		inCookieValue string
		outError      bool
	}{
		{
			desc:     "Missing cookie.",
			outError: true,
		},
		{
			desc:          "Invalid cookie.",
			inCookieValue: invalidCookieValue,
			outError:      true,
		},
		{
			desc:          "Valid cookie.",
			inCookieValue: validCookieValue,
			outError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			buffer, err := json.Marshal(&fragmentRequest{
				CookieValue: tt.inCookieValue,
			})
			assert.Nil(t, err)

			// Create POST request that will be sent to fragment handler endpoint.
			addr := pack.a.application.PublicAddr
			r, err := http.NewRequest(http.MethodPost, addr, bytes.NewReader(buffer))
			assert.Nil(t, err)

			// Make sure fragment handler only succeeds with valid session cookies.
			w := httptest.NewRecorder()
			err = handler.handleFragment(w, r)
			assert.Equal(t, err != nil, tt.outError)
			if tt.outError {
				return
			}

			// Check that the value returned in the "Set-Cookie" header matches the
			// value passed in.
			assert.Equal(t, http.StatusOK, w.Result().StatusCode)
			assert.Len(t, w.Result().Cookies(), 1)
			cookie := w.Result().Cookies()[0]
			assert.Equal(t, cookie.Name, cookieName)
			assert.Equal(t, cookie.Value, tt.inCookieValue)
		})
	}
}

// TestLogout verifies that logging out of the parent session logs all
// application specific child sessions out as well.
func TestLogout(t *testing.T) {
	pack, done := setup(t)
	defer done()

	// Create a web UI session.
	webSession, err := pack.s.client.CreateWebSession(pack.u.user.GetName())
	assert.Nil(t, err)

	// Use the Web UI session to ask for a application specific session.
	appSession, err := pack.u.client.CreateAppSession(context.Background(), services.CreateAppSessionRequest{
		AppName:     pack.a.application.Name,
		ClusterName: pack.s.tlsServer.ClusterName(),
		SessionID:   webSession.GetName(),
		BearerToken: webSession.GetBearerToken(),
	})
	assert.Nil(t, err)

	// Delete the parent session.
	err = pack.u.client.DeleteWebSession(pack.u.user.GetName(), webSession.GetName())
	assert.Nil(t, err)

	// Check that deleting the parent session removes the child session.
	_, err = pack.s.client.GetAppSession(context.Background(), services.GetAppSessionRequest{
		Username:   appSession.GetUser(),
		ParentHash: appSession.GetParentHash(),
		SessionID:  appSession.GetName(),
	})
	assert.Equal(t, trace.IsNotFound(err), true)
}

// TODO(russjones): Add a test where a request for multiple matching
// applications returns a 404.
func TestAmbiguous(t *testing.T) {
}

// TODO(russjones): Test connecting to an application through a trusted cluster.
func TestTrustedCluster(t *testing.T) {
}

type pack struct {
	clock clockwork.Clock

	s *authPack
	u *userPack
	p *proxyPack
	a *appPack
}

type authPack struct {
	client *auth.Client

	authServer *auth.TestAuthServer
	tlsServer  *auth.TestTLSServer

	authDir string
}

type userPack struct {
	user   services.User
	role   services.Role
	client *auth.Client
}

type proxyPack struct {
	client *auth.Client

	listener net.Listener
	tunnel   reversetunnel.Server

	handler *Handler

	ssh *regular.Server

	tunnelDir string
	proxyDir  string
}

type appPack struct {
	server      services.Server
	application *services.App
	client      *auth.Client
	appServer   *app.Server
	pool        *reversetunnel.AgentPool
}

func setup(t *testing.T) (*pack, func()) {
	utils.InitLoggerForTests(testing.Verbose())

	fakeClock := clockwork.NewFakeClockAt(time.Now())
	clusterName := "example.com"

	// Create components needed to run auth.
	auths, err := setupAuth(fakeClock, clusterName)
	assert.Nil(t, err)

	// Create user that will be making requests to the web application.
	user, err := setupUser(auths.tlsServer)
	assert.Nil(t, err)

	// Create components needed to run proxy.
	proxy, err := setupProxy(auths.tlsServer)
	assert.Nil(t, err)

	// Create internal application.
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, r.Header.Get("x-teleport-jwt-assertion"))
	}))
	//u, err := url.Parse(targetServer.URL)
	//assert.Nil(t, err)

	// Create components needed to run app proxy.
	app, err := setupApp(fakeClock, auths.tlsServer, proxy.listener.Addr().String(), targetServer.URL)
	assert.Nil(t, err)

	// Wait for the application to have registered itself with the proxy server
	// before exiting setup.
	err = waitForTunnelCount(proxy.tunnel, auths.tlsServer.ClusterName(), 1)
	assert.Nil(t, err)

	closeFunc := func() {
		os.RemoveAll(auths.authDir)
		os.RemoveAll(proxy.tunnelDir)
		os.RemoveAll(proxy.proxyDir)

		proxy.client.Close()
		proxy.listener.Close()
		proxy.tunnel.Close()
		proxy.ssh.Close()

		app.client.Close()
		app.appServer.Close()
		app.pool.Stop()
	}

	return &pack{
		clock: fakeClock,
		s:     auths,
		u:     user,
		p:     proxy,
		a:     app,
	}, closeFunc
}

func setupAuth(clock clockwork.FakeClock, clusterName string) (*authPack, error) {
	// Create a few temporary directories that will be removed at the end of the test.
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create and start auth.
	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Clock:       clock,
		ClusterName: clusterName,
		Dir:         dir,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsServer, err := authServer.NewTestTLSServer()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create client with role of proxy.
	authClient, err := tlsServer.NewClient(auth.TestBuiltin(teleport.RoleAdmin))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &authPack{
		client:     authClient,
		authServer: authServer,
		tlsServer:  tlsServer,
		authDir:    dir,
	}, nil
}

func setupUser(tlsServer *auth.TestTLSServer) (*userPack, error) {
	user, role, err := auth.CreateUserAndRole(tlsServer.Auth(), "foo", []string{"bar", "baz"})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := tlsServer.NewClient(auth.TestUser(user.GetName()))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &userPack{
		user:   user,
		role:   role,
		client: client,
	}, nil
}

func setupProxy(tlsServer *auth.TestTLSServer) (*proxyPack, error) {
	// Create a few temporary directories that will be removed at the end of the test.
	tunnelDir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyDir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create key and certificate.
	proxyUUID := uuid.New()
	proxyKeys, err := tlsServer.Auth().GenerateServerKeys(auth.GenerateServerKeysRequest{
		HostID:   proxyUUID,
		NodeName: tlsServer.ClusterName(),
		Roles:    teleport.Roles{teleport.RoleProxy},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxySigner, err := sshutils.NewSigner(proxyKeys.Key, proxyKeys.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create client with role of proxy.
	authClient, err := tlsServer.NewClient(auth.TestBuiltin(teleport.RoleProxy))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create and start the reverse tunnel server.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reverseTunnelServer, err := reversetunnel.NewServer(reversetunnel.Config{
		ClientTLS:             authClient.TLSConfig(),
		ID:                    proxyUUID,
		ClusterName:           tlsServer.ClusterName(),
		Listener:              listener,
		HostSigners:           []ssh.Signer{proxySigner},
		LocalAuthClient:       authClient,
		LocalAccessPoint:      authClient,
		NewCachingAccessPoint: auth.NoCache,
		DirectClusters: []reversetunnel.DirectCluster{
			reversetunnel.DirectCluster{
				Name:   tlsServer.ClusterName(),
				Client: authClient,
			},
		},
		DataDir:   tunnelDir,
		Component: teleport.ComponentProxy,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = reverseTunnelServer.Start()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create and start proxy SSH server.
	sshServer, err := regular.New(
		utils.NetAddr{
			AddrNetwork: "tcp",
			Addr:        "127.0.0.1:0",
		},
		tlsServer.ClusterName(),
		[]ssh.Signer{proxySigner},
		authClient,
		proxyDir,
		"",
		utils.NetAddr{},
		regular.SetUUID(proxyUUID),
		regular.SetProxyMode(reverseTunnelServer),
		regular.SetSessionServer(authClient),
		regular.SetAuditLog(authClient),
		regular.SetNamespace(defaults.Namespace),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = sshServer.Start()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proxyPack{
		client:    authClient,
		listener:  listener,
		tunnel:    reverseTunnelServer,
		ssh:       sshServer,
		tunnelDir: tunnelDir,
		proxyDir:  proxyDir,
	}, nil
}

func setupApp(clock clockwork.Clock, tlsServer *auth.TestTLSServer, proxyAddr string, targetAddr string) (*appPack, error) {
	// Generate key and certificate.
	appUUID := uuid.New()
	appKeys, err := tlsServer.Auth().GenerateServerKeys(auth.GenerateServerKeysRequest{
		HostID:   appUUID,
		NodeName: tlsServer.ClusterName(),
		Roles:    teleport.Roles{teleport.RoleApp},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	appSigner, err := sshutils.NewSigner(appKeys.Key, appKeys.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create client with role of app.
	authClient, err := tlsServer.NewClient(auth.TestBuiltin(teleport.RoleApp))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	application := &services.App{
		Name:       "panel",
		URI:        targetAddr,
		PublicAddr: "panel.example.com",
	}
	server := &services.ServerV2{
		Kind:    services.KindApp,
		Version: services.V2,
		Metadata: services.Metadata{
			Namespace: defaults.Namespace,
			Name:      appUUID,
		},
		Spec: services.ServerSpecV2{
			Protocol: services.ServerSpecV2_HTTPS,
			Version:  teleport.Version,
			Apps: []*services.App{
				application,
			},
		},
	}

	// Create and start application proxy server.
	appServer, err := app.New(context.Background(), &app.Config{
		Clock:       clock,
		AccessPoint: authClient,
		GetRotation: testRotationGetter,
		Server:      server,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	appServer.Start()
	err = appServer.ForceHeartbeat()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create and establish reverse tunnel.
	agentPool, err := reversetunnel.NewAgentPool(reversetunnel.AgentPoolConfig{
		Component:   teleport.ComponentApp,
		HostUUID:    fmt.Sprintf("%v.%v", appUUID, tlsServer.ClusterName()),
		ProxyAddr:   proxyAddr,
		Client:      authClient,
		AppServer:   appServer,
		AccessPoint: authClient,
		HostSigners: []ssh.Signer{appSigner},
		Cluster:     tlsServer.ClusterName(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = agentPool.Start()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &appPack{
		application: application,
		server:      server,
		client:      authClient,
		appServer:   appServer,
		pool:        agentPool,
	}, nil
}

func testRotationGetter(role teleport.Role) (*services.Rotation, error) {
	return &services.Rotation{}, nil
}

func waitForTunnelCount(tunnel reversetunnel.Server, clusterName string, expected int) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(10 * time.Second)
	defer ticker.Stop()

	clusterClient, err := tunnel.GetSite(clusterName)
	if err != nil {
		return trace.Wrap(err)
	}

	for {
		select {
		case <-ticker.C:
			if clusterClient.GetTunnelsCount() == expected {
				return nil
			}
		case <-timeout.C:
			return trace.BadParameter("timed out waiting for tunnel count")
		}
	}
}
