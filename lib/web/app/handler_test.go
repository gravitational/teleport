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

func TestForward(t *testing.T) {
	// Setup auth, proxy, and app components for test.
	pack, done := setup(t)
	defer done()

	time.Sleep(10 * time.Second)

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

type pack struct {
	clock clockwork.Clock

	tlsServer  *auth.TestTLSServer
	authServer *auth.TestAuthServer

	proxyClient  *auth.Client
	proxyReverse reversetunnel.Server
	proxyHandler *Handler

	appClient *auth.Client
	appServer *app.Server
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

	/// Create components needed to run proxy.
	proxyClient, proxyReverse, proxyListener, proxySSH, proxyHandler, err := setupProxy(tlsServer, tunnelDir, proxyDir)
	assert.Nil(t, err)

	// Create internal application.
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello, world")
	}))

	// Create components needed to run app proxy.
	appClient, appServer, appPool, err := setupApp(fakeClock, tlsServer, proxyListener.Addr().String(), targetServer.URL)
	assert.Nil(t, err)

	closeFunc := func() {
		os.RemoveAll(authDir)
		os.RemoveAll(tunnelDir)
		os.RemoveAll(proxyDir)

		proxyClient.Close()
		proxyListener.Close()
		proxyReverse.Close()
		proxySSH.Close()

		appClient.Close()
		appServer.Close()
		appPool.Stop()
	}

	return &pack{
		clock: fakeClock,

		tlsServer: tlsServer,

		proxyClient:  proxyClient,
		proxyReverse: proxyReverse,
		proxyHandler: proxyHandler,

		appClient: appClient,
		appServer: appServer,
	}, closeFunc
}

func setupProxy(tlsServer *auth.TestTLSServer, tunnelDir string, proxyDir string) (*auth.Client, reversetunnel.Server, net.Listener, *regular.Server, *Handler, error) {
	// Create key and certificate.
	proxyUUID := uuid.New()
	proxyKeys, err := tlsServer.Auth().GenerateServerKeys(auth.GenerateServerKeysRequest{
		HostID:   proxyUUID,
		NodeName: tlsServer.ClusterName(),
		Roles:    teleport.Roles{teleport.RoleProxy},
	})
	if err != nil {
		return nil, nil, nil, nil, nil, trace.Wrap(err)
	}
	proxySigner, err := sshutils.NewSigner(proxyKeys.Key, proxyKeys.Cert)
	if err != nil {
		return nil, nil, nil, nil, nil, trace.Wrap(err)
	}

	// Create client with role of proxy.
	authClient, err := tlsServer.NewClient(auth.TestBuiltin(teleport.RoleProxy))
	if err != nil {
		return nil, nil, nil, nil, nil, trace.Wrap(err)
	}

	// Create and start the reverse tunnel server.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, nil, nil, nil, trace.Wrap(err)
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
		return nil, nil, nil, nil, nil, trace.Wrap(err)
	}
	err = reverseTunnelServer.Start()
	if err != nil {
		return nil, nil, nil, nil, nil, trace.Wrap(err)
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
		return nil, nil, nil, nil, nil, trace.Wrap(err)
	}
	err = sshServer.Start()
	if err != nil {
		return nil, nil, nil, nil, nil, trace.Wrap(err)
	}

	// Create application handler.
	handler, err := NewHandler(HandlerConfig{
		AuthClient:  authClient,
		ProxyClient: reverseTunnelServer,
	})
	if err != nil {
		return nil, nil, nil, nil, nil, trace.Wrap(err)
	}

	return authClient, reverseTunnelServer, listener, sshServer, handler, nil
}

func setupApp(clock clockwork.Clock, tlsServer *auth.TestTLSServer, proxyAddr string, targetAddr string) (*auth.Client, *app.Server, *reversetunnel.AgentPool, error) {
	// Generate key and certificate.
	appUUID := uuid.New()
	appKeys, err := tlsServer.Auth().GenerateServerKeys(auth.GenerateServerKeysRequest{
		HostID:   appUUID,
		NodeName: tlsServer.ClusterName(),
		Roles:    teleport.Roles{teleport.RoleApp},
	})
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	appSigner, err := sshutils.NewSigner(appKeys.Key, appKeys.Cert)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	// Create client with role of app.
	authClient, err := tlsServer.NewClient(auth.TestBuiltin(teleport.RoleApp))
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	// Create and start application proxy server.
	appServer, err := app.New(context.Background(), &app.Config{
		Clock:       clock,
		AccessPoint: authClient,
		GetRotation: testRotationGetter,
		App: &services.ServerV2{
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
		},
	})
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	appServer.Start()
	err = appServer.ForceHeartbeat()
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
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
		return nil, nil, nil, trace.Wrap(err)
	}
	err = agentPool.Start()
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	return authClient, appServer, agentPool, nil
}

func testRotationGetter(role teleport.Role) (*services.Rotation, error) {
	return &services.Rotation{}, nil
}
