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

package integration

import (
	"bytes"
	"context"
	"net"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/require"
)

type proxyPeer struct {
	username string
	cluster  string
	authAddr *utils.NetAddr
	lbAddr   *utils.NetAddr
}

func TestPeerProxyAgentMesh(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	// load balancer for proxies.
	lbAddr := utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(ports.PopInt())))
	lb, err := utils.NewLoadBalancer(context.TODO(), *lbAddr)
	require.NoError(t, err)

	p := &proxyPeer{
		username: mustGetCurrentUser(t).Username,
		cluster:  "agent-mesh-proxy-peer",
		lbAddr:   lbAddr,
	}

	// boostrap an auth instance
	auth, authConf := p.makeAuth(t)
	// TODO update autConf with agent-mesh config.
	// it should not affect the outcome of this test.
	require.NoError(t, auth.CreateEx(t, nil, authConf))

	// boostrap a first proxy instance.
	proxy1, proxyConf1 := p.makeProxy(t)
	require.NoError(t, proxy1.CreateEx(t, nil, proxyConf1))
	lb.AddBackend(proxyConf1.Proxy.WebAddr)

	// boostrap a second proxy instance.
	proxy2, proxyConf2 := p.makeProxy(t)
	require.NoError(t, proxy2.CreateEx(t, nil, proxyConf2))
	lb.AddBackend(proxyConf2.Proxy.WebAddr)

	// boostrap a node instance.
	node, nodeConf := p.makeNode(t)
	require.NoError(t, node.CreateEx(t, nil, nodeConf))

	t.Cleanup(func() {
		node.StopAll()
		proxy2.StopAll()
		proxy1.StopAll()
		auth.StopAll()
		lb.Close()
	})

	// start the load balancer.
	require.NoError(t, lb.Listen())
	go lb.Serve()

	// start the instances.
	require.NoError(t, auth.Start())
	require.NoError(t, proxy1.Start())
	require.NoError(t, proxy2.Start())
	require.NoError(t, node.Start())

	// wait for the node to open reverse tunnels to both proxies.
	waitForActiveTunnelConnections(t, proxy1.Tunnel, p.cluster, 1)
	waitForActiveTunnelConnections(t, proxy2.Tunnel, p.cluster, 1)

	// make sure we can connect to the node going though any proxy.
	p.dialNode(t, auth, proxy1, node)
	p.dialNode(t, auth, proxy2, node)
}

// dialNode starts a client conn to a node reachable through a specific proxy.
func (p *proxyPeer) dialNode(t *testing.T, auth, proxy, node *TeleInstance) {
	ident, err := node.Process.GetIdentity(types.RoleNode)
	require.NoError(t, err)
	nodeuuid, err := ident.ID.HostID()
	require.NoError(t, err)

	creds, err := GenerateUserCreds(UserCredsRequest{
		Process:  auth.Process,
		Username: p.username,
	})
	require.NoError(t, err)

	client, err := proxy.NewClientWithCreds(
		ClientConfig{
			Cluster: p.cluster,
			Host:    nodeuuid,
		},
		*creds,
	)
	require.NoError(t, err)

	output := &bytes.Buffer{}
	client.Stdout = output

	cmd := []string{"echo", "hello world"}
	err = client.SSH(context.Background(), cmd, false)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output.String())
}

// makeAuth bootsraps a new teleport auth instance.
func (p *proxyPeer) makeAuth(t *testing.T) (*TeleInstance, *service.Config) {
	privateKey, publicKey, err := testauthority.New().GenerateKeyPair("")
	require.NoError(t, err)

	auth := NewInstance(InstanceConfig{
		ClusterName: p.cluster,
		HostID:      uuid.New().String(),
		NodeName:    Loopback,
		Priv:        privateKey,
		Pub:         publicKey,
		log:         utils.NewLoggerForTests(),
	})

	auth.AddUser(p.username, []string{p.username})

	conf := service.MakeDefaultConfig()
	conf.DataDir = t.TempDir()
	conf.Auth.Enabled = true
	conf.Proxy.Enabled = false
	conf.SSH.Enabled = false

	p.authAddr = utils.MustParseAddr(net.JoinHostPort(auth.Hostname, auth.GetPortAuth()))

	return auth, conf
}

// makeProxy boostraps a new teleport proxy instance.
// It's public address points to a load balancer.
func (p *proxyPeer) makeProxy(t *testing.T) (*TeleInstance, *service.Config) {
	proxy := NewInstance(InstanceConfig{
		ClusterName: p.cluster,
		HostID:      uuid.New().String(),
		NodeName:    Loopback,
		log:         utils.NewLoggerForTests(),
	})

	conf := service.MakeDefaultConfig()
	conf.AuthServers = append(conf.AuthServers, *p.authAddr)
	conf.Token = "token"
	conf.DataDir = t.TempDir()

	conf.Auth.Enabled = false
	conf.SSH.Enabled = false

	conf.Proxy.Enabled = true
	conf.Proxy.WebAddr.Addr = net.JoinHostPort(Loopback, strconv.Itoa(ports.PopInt()))
	conf.Proxy.PublicAddrs = append(conf.Proxy.PublicAddrs, *p.lbAddr)
	conf.Proxy.DisableWebInterface = true

	return proxy, conf
}

// makeNode boostraps a new teleport node instance.
// It connects to a proxy via a reverse tunnel going through a load balancer.
func (p *proxyPeer) makeNode(t *testing.T) (*TeleInstance, *service.Config) {
	node := NewInstance(InstanceConfig{
		ClusterName: p.cluster,
		HostID:      uuid.New().String(),
		NodeName:    Loopback,
		log:         utils.NewLoggerForTests(),
	})

	conf := service.MakeDefaultConfig()
	conf.AuthServers = append(conf.AuthServers, *p.lbAddr)
	conf.Token = "token"
	conf.DataDir = t.TempDir()

	conf.Auth.Enabled = false
	conf.Proxy.Enabled = false
	conf.SSH.Enabled = true

	return node, conf
}
