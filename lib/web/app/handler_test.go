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
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
)

func TestFoo1(t *testing.T) {
	_, done := setup(t)
	defer done()
}

type pack struct {
	clock clockwork.Clock

	tlsServer  *auth.TestTLSServer
	authServer *auth.TestAuthServer

	reverseTunnelServer reversetunnel.Server
}

func setup(t *testing.T) (*pack, func()) {
	utils.InitLoggerForTests(testing.Verbose())

	fakeClock := clockwork.NewFakeClock()

	//authUUID := "00000000-0000-0000-0000-000000000000"
	//proxyUUID := "00000000-0000-0000-0000-000000000000"
	//appUUID := "00000000-0000-0000-0000-000000000000"

	authUUID := uuid.New()
	proxyUUID := uuid.New()
	appUUID := uuid.New()

	fmt.Printf("--> authUUID: %v.\n", authUUID)
	fmt.Printf("--> proxyUUID: %v.\n", proxyUUID)
	fmt.Printf("--> appUUID: %v.\n", appUUID)

	authDir, err := ioutil.TempDir("", "")
	assert.Nil(t, err)
	tunnelDir, err := ioutil.TempDir("", "")
	assert.Nil(t, err)
	proxyDir, err := ioutil.TempDir("", "")
	assert.Nil(t, err)

	// Create test auth server.
	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		ClusterName: "example.com",
		Dir:         authDir,
	})
	assert.Nil(t, err)
	tlsServer, err := authServer.NewTestTLSServer()
	assert.Nil(t, err)

	// Generate host key and certificate for proxy server.
	proxyKeys, err := tlsServer.Auth().GenerateServerKeys(auth.GenerateServerKeysRequest{
		HostID:   proxyUUID,
		NodeName: tlsServer.ClusterName(),
		Roles:    teleport.Roles{teleport.RoleProxy},
	})
	assert.Nil(t, err)
	proxySigner, err := sshutils.NewSigner(proxyKeys.Key, proxyKeys.Cert)
	assert.Nil(t, err)

	// Generate host key and certificate for proxy server.
	appKeys, err := tlsServer.Auth().GenerateServerKeys(auth.GenerateServerKeysRequest{
		HostID:   appUUID,
		NodeName: tlsServer.ClusterName(),
		Roles:    teleport.Roles{teleport.RoleApp},
	})
	assert.Nil(t, err)
	appSigner, err := sshutils.NewSigner(appKeys.Key, appKeys.Cert)
	assert.Nil(t, err)

	// Create a listener for the reverse tunnel server.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.Nil(t, err)
	defer listener.Close()

	// Create a few clients to the auth server.
	proxyAuthClient, err := tlsServer.NewClient(auth.TestBuiltin(teleport.RoleProxy))
	assert.Nil(t, err)
	appAuthClient, err := tlsServer.NewClient(auth.TestBuiltin(teleport.RoleApp))
	assert.Nil(t, err)

	// Create and start the reverse tunnel server.
	reverseTunnelServer, err := reversetunnel.NewServer(reversetunnel.Config{
		ClientTLS:             proxyAuthClient.TLSConfig(),
		ID:                    proxyUUID,
		ClusterName:           tlsServer.ClusterName(),
		Listener:              listener,
		HostSigners:           []ssh.Signer{proxySigner},
		LocalAuthClient:       proxyAuthClient,
		LocalAccessPoint:      proxyAuthClient,
		NewCachingAccessPoint: auth.NoCache,
		DirectClusters: []reversetunnel.DirectCluster{
			reversetunnel.DirectCluster{
				Name:   tlsServer.ClusterName(),
				Client: proxyAuthClient,
			},
		},
		DataDir:   tunnelDir,
		Component: teleport.ComponentProxy,
	})
	assert.Nil(t, err)
	err = reverseTunnelServer.Start()
	assert.Nil(t, err)

	proxy, err := regular.New(
		utils.NetAddr{
			AddrNetwork: "tcp",
			Addr:        "127.0.0.1:0",
		},
		tlsServer.ClusterName(),
		[]ssh.Signer{proxySigner},
		proxyAuthClient,
		proxyDir,
		"",
		utils.NetAddr{},
		regular.SetUUID(proxyUUID),
		regular.SetProxyMode(reverseTunnelServer),
		regular.SetSessionServer(proxyAuthClient),
		regular.SetAuditLog(proxyAuthClient),
		regular.SetNamespace(defaults.Namespace),
		//SetPAMConfig(&pam.Config{Enabled: false}),
		//SetBPF(&bpf.NOP{}),
	)
	assert.Nil(t, err)
	err = proxy.Start()
	assert.Nil(t, err)

	//tunnelAddr := utils.NetAddr{
	//	AddrNetwork: "tcp",
	//	Addr:        listener.Addr().String(),
	//}

	testWebServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello, world")
	}))

	//// Extract the hostport that the in-memory HTTP server is running on.
	//u, err := url.Parse(s.testhttp.URL)
	//c.Assert(err, check.IsNil)
	//s.hostport = u.Host

	appServer, err := app.New(context.Background(), &app.Config{
		Clock:       fakeClock,
		AccessPoint: appAuthClient,
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
				InternalAddr: testWebServer.URL,
				PublicAddr:   "panel.example.com",
				Version:      teleport.Version,
			},
		},
	})
	assert.Nil(t, err)

	appServer.Start()
	assert.Nil(t, err)

	agentPool, err := reversetunnel.NewAgentPool(reversetunnel.AgentPoolConfig{
		Component:   teleport.ComponentApp,
		HostUUID:    fmt.Sprintf("%v.%v", appUUID, tlsServer.ClusterName()),
		ProxyAddr:   listener.Addr().String(),
		Client:      appAuthClient,
		AppServer:   appServer,
		AccessPoint: appAuthClient,
		HostSigners: []ssh.Signer{appSigner},
		Cluster:     "example.com",
	})
	assert.Nil(t, err)

	err = agentPool.Start()
	assert.Nil(t, err)

	time.Sleep(20 * time.Second)

	closeFunc := func() {
		os.RemoveAll(authDir)
		os.RemoveAll(tunnelDir)
		os.RemoveAll(proxyDir)
	}

	return &pack{
		clock: fakeClock,
	}, closeFunc
}

func testRotationGetter(role teleport.Role) (*services.Rotation, error) {
	return &services.Rotation{}, nil
}
