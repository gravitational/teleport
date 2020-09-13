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

func TestAuthenticate(t *testing.T) {
	pack, done := setup(t)
	defer done()

	// Create application handler.
	handler, err := NewHandler(HandlerConfig{
		AuthClient:  pack.proxy.client,
		ProxyClient: pack.proxy.tunnel,
	})
	assert.Nil(t, err)

	user, _, err := auth.CreateUserAndRole(pack.tlsServer.Auth(), "foo", []string{"bar", "baz"})
	assert.Nil(t, err)

	webSession, err := pack.authClient.CreateWebSession(user.GetName())
	assert.Nil(t, err)

	userClient, err := pack.tlsServer.NewClient(auth.TestUser(user.GetName()))
	assert.Nil(t, err)

	appSession, err := userClient.CreateAppSession(context.Background(), services.CreateAppSessionRequest{
		AppName:     pack.app.app.GetAppName(),
		ClusterName: pack.tlsServer.ClusterName(),
		SessionID:   webSession.GetName(),
		BearerToken: webSession.GetBearerToken(),
	})
	assert.Nil(t, err)

	validCookieValue, err := encodeCookie(&Cookie{
		Username:   user.GetName(),
		ParentHash: appSession.GetParentHash(),
		SessionID:  appSession.GetName(),
	})
	assert.Nil(t, err)
	//invalidCookieValue, err := encodeCookie(&Cookie{
	//	Username:   user.GetName(),
	//	ParentHash: appSession.GetParentHash(),
	//	SessionID:  "invalid-session-id",
	//})
	//assert.Nil(t, err)

	var tests = []struct {
		desc     string
		inCookie *http.Cookie
		outError bool
	}{
		//{
		//	desc:     "Missing headers.",
		//	inCookie: nil,
		//	outError: true,
		//},
		{
			desc: "Valid.",
			inCookie: &http.Cookie{
				Name:  cookieName,
				Value: validCookieValue,
			},
			outError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Create a request without any authentication headers.
			r, err := http.NewRequest(http.MethodGet, "/", nil)
			assert.Nil(t, err)

			if tt.inCookie != nil {
				r.AddCookie(tt.inCookie)
			}

			_, err = handler.authenticate(r)
			fmt.Printf("--> err: %v.\n", err)
			assert.Equal(t, tt.outError, err != nil)
		})
	}
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

	authClient *auth.Client
	authServer *auth.TestAuthServer
	tlsServer  *auth.TestTLSServer

	proxy *proxyPack
	app   *appPack
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

	// Create a few temporary directories that will be removed at the end of the test.
	authDir, err := ioutil.TempDir("", "")
	assert.Nil(t, err)
	tunnelDir, err := ioutil.TempDir("", "")
	assert.Nil(t, err)
	proxyDir, err := ioutil.TempDir("", "")
	assert.Nil(t, err)

	// Create auth.
	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Clock:       fakeClock,
		ClusterName: "example.com",
		Dir:         authDir,
	})
	assert.Nil(t, err)
	tlsServer, err := authServer.NewTestTLSServer()
	assert.Nil(t, err)

	// Create client with role of proxy.
	authClient, err := tlsServer.NewClient(auth.TestBuiltin(teleport.RoleAdmin))
	assert.Nil(t, err)

	// Create components needed to run proxy.
	proxy, err := setupProxy(tlsServer, tunnelDir, proxyDir)
	assert.Nil(t, err)

	// Create internal application.
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello, world")
	}))
	u, err := url.Parse(targetServer.URL)
	assert.Nil(t, err)

	// Create components needed to run app proxy.
	app, err := setupApp(fakeClock, tlsServer, proxy.listener.Addr().String(), u.Host)
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
		clock:      fakeClock,
		authClient: authClient,
		authServer: authServer,
		tlsServer:  tlsServer,
		proxy:      proxy,
		app:        app,
	}, closeFunc
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
