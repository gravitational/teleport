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
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	authClient, err := pack.auth.tlsServer.NewClient(auth.TestBuiltin(teleport.RoleWeb))
	assert.Nil(t, err)
	handler, err := NewHandler(HandlerConfig{
		AuthClient:  authClient,
		ProxyClient: pack.proxy.tunnel,
	})
	assert.Nil(t, err)

	// Create a web UI session.
	webSession, err := pack.auth.client.CreateWebSession(pack.user.user.GetName())
	assert.Nil(t, err)

	// Use the Web UI session to ask for a application specific session.
	appSession, err := pack.user.client.CreateAppSession(context.Background(), services.CreateAppSessionRequest{
		AppName:     pack.app.app.GetAppName(),
		ClusterName: pack.auth.tlsServer.ClusterName(),
		SessionID:   webSession.GetName(),
		BearerToken: webSession.GetBearerToken(),
	})
	assert.Nil(t, err)

	// Create a few cookie values to test.
	validCookieValue, err := encodeCookie(&Cookie{
		Username:   pack.user.user.GetName(),
		ParentHash: appSession.GetParentHash(),
		SessionID:  appSession.GetName(),
	})
	assert.Nil(t, err)
	invalidCookieValue, err := encodeCookie(&Cookie{
		Username:   pack.user.user.GetName(),
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

func TestForward(t *testing.T) {
	pack, done := setup(t)
	defer done()

	// Create application handler that runs within the web handlers.
	authClient, err := pack.auth.tlsServer.NewClient(auth.TestBuiltin(teleport.RoleWeb))
	assert.Nil(t, err)
	handler, err := NewHandler(HandlerConfig{
		AuthClient:  authClient,
		ProxyClient: pack.proxy.tunnel,
	})
	assert.Nil(t, err)

	// Create a web UI session.
	webSession, err := pack.auth.client.CreateWebSession(pack.user.user.GetName())
	assert.Nil(t, err)

	// Use the Web UI session to ask for a application specific session.
	appSession, err := pack.user.client.CreateAppSession(context.Background(), services.CreateAppSessionRequest{
		AppName:     pack.app.app.GetAppName(),
		ClusterName: pack.auth.tlsServer.ClusterName(),
		SessionID:   webSession.GetName(),
		BearerToken: webSession.GetBearerToken(),
	})
	assert.Nil(t, err)

	// Create a session cache and create a session.
	cache, err := newSessionCache(sessionCacheConfig{
		AuthClient:  authClient,
		ProxyClient: pack.proxy.tunnel,
	})
	assert.Nil(t, err)
	session, err := cache.newSession(context.Background(), "not-required", appSession)
	assert.Nil(t, err)

	r, err := http.NewRequest(http.MethodGet, pack.app.app.GetPublicAddr(), nil)
	assert.Nil(t, err)
	w := httptest.NewRecorder()

	err = handler.forward(w, r, session)
	assert.Nil(t, err)

	resp := w.Result()
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	assert.Nil(t, err)

	fmt.Printf("--> code: %v.\n", resp.StatusCode)
	fmt.Printf("--> body: %v.\n", string(body))
}

/*
func TestAuthenticate(t *testing.T) {
	pack, done := setup(t)
	defer done()

	// Create application handler.
	handler, err := NewHandler(HandlerConfig{
		AuthClient:  authClient,
		ProxyClient: reverseTunnelServer,
	})
	if err != nil {
		return nil, nil, nil, nil, nil, trace.Wrap(err)
	}

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pack.proxyHandler.ServeHTTP(w, r)
	}))

	res, err := http.Get(webServer.URL)
	assert.Nil(t, err)
	greeting, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	assert.Nil(t, err)

	fmt.Printf("--> %v", string(greeting))
}
*/

type pack struct {
	clock clockwork.Clock

	auth  *authPack
	user  *userPack
	proxy *proxyPack
	app   *appPack
}

type authPack struct {
	client     *auth.Client
	authServer *auth.TestAuthServer
	tlsServer  *auth.TestTLSServer
}

type userPack struct {
	user   services.User
	role   services.Role
	client *auth.Client
}

type proxyPack struct {
	client   *auth.Client
	listener net.Listener
	tunnel   reversetunnel.Server
	handler  *Handler
	ssh      *regular.Server
}

type appPack struct {
	app    services.Server
	client *auth.Client
	server *app.Server
	pool   *reversetunnel.AgentPool
}

func setup(t *testing.T) (*pack, func()) {
	utils.InitLoggerForTests(testing.Verbose())

	fakeClock := clockwork.NewFakeClockAt(time.Now())
	clusterName := "example.com"

	// Create a few temporary directories that will be removed at the end of the test.
	authDir, err := ioutil.TempDir("", "")
	assert.Nil(t, err)
	tunnelDir, err := ioutil.TempDir("", "")
	assert.Nil(t, err)
	proxyDir, err := ioutil.TempDir("", "")
	assert.Nil(t, err)

	// Create components needed to run auth.
	auth, err := setupAuth(fakeClock, clusterName, authDir)
	assert.Nil(t, err)

	// Create user that will be making requests to the web application.
	user, err := setupUser(auth.tlsServer)
	assert.Nil(t, err)

	// Create components needed to run proxy.
	proxy, err := setupProxy(auth.tlsServer, tunnelDir, proxyDir)
	assert.Nil(t, err)

	// Create internal application.
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello, world")
	}))
	u, err := url.Parse(targetServer.URL)
	assert.Nil(t, err)

	// Create components needed to run app proxy.
	app, err := setupApp(fakeClock, auth.tlsServer, proxy.listener.Addr().String(), u.Host)
	assert.Nil(t, err)

	// Wait for the application to have registered itself with the proxy server
	// before exiting setup.
	err = waitForTunnelCount(proxy.tunnel, auth.tlsServer.ClusterName(), 1)
	assert.Nil(t, err)

	closeFunc := func() {
		os.RemoveAll(authDir)
		os.RemoveAll(tunnelDir)
		os.RemoveAll(proxyDir)

		proxy.client.Close()
		proxy.listener.Close()
		proxy.tunnel.Close()
		proxy.ssh.Close()

		app.client.Close()
		app.server.Close()
		app.pool.Stop()
	}

	return &pack{
		clock: fakeClock,
		auth:  auth,
		user:  user,
		proxy: proxy,
		app:   app,
	}, closeFunc
}

func setupAuth(clock clockwork.FakeClock, clusterName string, dir string) (*authPack, error) {
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

func setupProxy(tlsServer *auth.TestTLSServer, tunnelDir string, proxyDir string) (*proxyPack, error) {
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
		client:   authClient,
		listener: listener,
		tunnel:   reverseTunnelServer,
		ssh:      sshServer,
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

	application := &services.ServerV2{
		Kind:    services.KindApp,
		Version: services.V2,
		Metadata: services.Metadata{
			Namespace: defaults.Namespace,
			Name:      appUUID,
		},
		Spec: services.ServerSpecV2{
			Protocol:     services.ServerSpecV2_HTTPS,
			InternalAddr: targetAddr,
			AppName:      "panel",
			PublicAddr:   "panel.example.com",
			Version:      teleport.Version,
		},
	}

	// Create and start application proxy server.
	appServer, err := app.New(context.Background(), &app.Config{
		Clock:       clock,
		AccessPoint: authClient,
		GetRotation: testRotationGetter,
		App:         application,
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
		app:    application,
		client: authClient,
		server: appServer,
		pool:   agentPool,
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
