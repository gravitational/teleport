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

type proxyTunnelStrategy struct {
	username string
	cluster  string

	lb     *utils.LoadBalancer
	auth   *TeleInstance
	proxy1 *TeleInstance
	proxy2 *TeleInstance
	node   *TeleInstance
}

// TestProxyTunnelStrategyAgentMesh tests the agent-mesh tunnel strategy
func TestProxyTunnelStrategyAgentMesh(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	p := &proxyTunnelStrategy{
		cluster:  "proxy-tunnel-agent-mesh",
		username: mustGetCurrentUser(t).Username,
	}

	strategy := &types.TunnelStrategyV1{
		Strategy: &types.TunnelStrategyV1_AgentMesh{
			AgentMesh: types.DefaultAgentMeshTunnelStrategy(),
		},
	}

	// boostrap an load balancer for proxies.
	p.makeLoadBalancer(t)

	// boostrap an auth instance.
	p.makeAuth(t, strategy)

	// boostrap two proxy instances.
	p.makeProxy(t)
	p.makeProxy(t)

	// boostrap a node instance.
	p.makeNode(t)

	// wait for the node to open reverse tunnels to both proxies.
	waitForActiveTunnelConnections(t, p.proxy1.Tunnel, p.cluster, 1)
	waitForActiveTunnelConnections(t, p.proxy2.Tunnel, p.cluster, 1)

	// make sure we can connect to the node going though any proxy.
	p.dialNode(t, p.auth, p.proxy1, p.node)
	p.dialNode(t, p.auth, p.proxy2, p.node)
}

// TestProxyTunnelStrategyProxyPeering tests the proxy-peer tunnel strategy
func TestProxyTunnelStrategyProxyPeering(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	p := &proxyTunnelStrategy{
		cluster:  "proxy-tunnel-proxy-peer",
		username: mustGetCurrentUser(t).Username,
	}

	strategy := &types.TunnelStrategyV1{
		Strategy: &types.TunnelStrategyV1_ProxyPeering{
			ProxyPeering: types.DefaultProxyPeeringTunnelStrategy(),
		},
	}

	// boostrap an load balancer for proxies.
	p.makeLoadBalancer(t)

	// boostrap an auth instance.
	p.makeAuth(t, strategy)

	// boostrap the first proxy instance.
	p.makeProxy(t)

	// boostrap a node instance.
	p.makeNode(t)

	// wait for the node to open a reverse tunnel to the first proxy.
	waitForActiveTunnelConnections(t, p.proxy1.Tunnel, p.cluster, 1)

	// boostrap the second proxy instance after the node has already established
	// a reverse tunnel to the first proxy.
	p.makeProxy(t)

	// make sure node doesn't open any reverse tunnel to the second proxy.
	waitForMaxActiveTunnelConnections(t, p.proxy2.Tunnel, p.cluster, 0)

	// make sure we can connect to the node going though any proxy.
	p.dialNode(t, p.auth, p.proxy1, p.node)
	p.dialNode(t, p.auth, p.proxy2, p.node)
}

// dialNode starts a client conn to a node reachable through a specific proxy.
func (p *proxyTunnelStrategy) dialNode(t *testing.T, auth, proxy, node *TeleInstance) {
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

// makeAuth bootsraps a new load balancer for proxy instances.
func (p *proxyTunnelStrategy) makeLoadBalancer(t *testing.T) {
	lbAddr := utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(ports.PopInt())))
	lb, err := utils.NewLoadBalancer(context.TODO(), *lbAddr)
	require.NoError(t, err)

	t.Cleanup(func() {
		lb.Close()
	})

	require.NoError(t, lb.Listen())
	go lb.Serve()

	if p.lb == nil {
		p.lb = lb
	} else {
		t.Error("load balancer already initialized")
	}
}

// makeAuth bootsraps a new teleport auth instance.
func (p *proxyTunnelStrategy) makeAuth(t *testing.T, strategy *types.TunnelStrategyV1) {
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
	conf.Auth.NetworkingConfig.SetTunnelStrategy(strategy)
	conf.Proxy.Enabled = false
	conf.SSH.Enabled = false

	require.NoError(t, auth.CreateEx(t, nil, conf))

	t.Cleanup(func() {
		auth.StopAll()
	})
	require.NoError(t, auth.Start())

	if p.auth == nil {
		p.auth = auth
	} else {
		t.Error("auth already initialized")
	}
}

// makeProxy boostraps a new teleport proxy instance.
// It's public address points to a load balancer.
func (p *proxyTunnelStrategy) makeProxy(t *testing.T) {
	proxy := NewInstance(InstanceConfig{
		ClusterName: p.cluster,
		HostID:      uuid.New().String(),
		NodeName:    Loopback,
		log:         utils.NewLoggerForTests(),
	})

	authAddr := utils.MustParseAddr(net.JoinHostPort(p.auth.Hostname, p.auth.GetPortAuth()))

	conf := service.MakeDefaultConfig()
	conf.AuthServers = append(conf.AuthServers, *authAddr)
	conf.Token = "token"
	conf.DataDir = t.TempDir()

	conf.Auth.Enabled = false
	conf.SSH.Enabled = false

	conf.Proxy.Enabled = true
	conf.Proxy.WebAddr.Addr = net.JoinHostPort(Loopback, strconv.Itoa(ports.PopInt()))
	conf.Proxy.PeerAddr.Addr = net.JoinHostPort(Loopback, strconv.Itoa(ports.PopInt()))
	conf.Proxy.PublicAddrs = append(conf.Proxy.PublicAddrs, utils.FromAddr(p.lb.Addr()))
	conf.Proxy.DisableWebInterface = true

	require.NoError(t, proxy.CreateEx(t, nil, conf))
	p.lb.AddBackend(conf.Proxy.WebAddr)

	t.Cleanup(func() {
		proxy.StopAll()
	})
	require.NoError(t, proxy.Start())

	if p.proxy1 == nil {
		p.proxy1 = proxy
	} else if p.proxy2 == nil {
		p.proxy2 = proxy
	} else {
		t.Error("both proxies already initialized")
	}
}

// makeNode boostraps a new teleport node instance.
// It connects to a proxy via a reverse tunnel going through a load balancer.
func (p *proxyTunnelStrategy) makeNode(t *testing.T) {
	node := NewInstance(InstanceConfig{
		ClusterName: p.cluster,
		HostID:      uuid.New().String(),
		NodeName:    Loopback,
		log:         utils.NewLoggerForTests(),
	})

	conf := service.MakeDefaultConfig()
	conf.AuthServers = append(conf.AuthServers, utils.FromAddr(p.lb.Addr()))
	conf.Token = "token"
	conf.DataDir = t.TempDir()

	conf.Auth.Enabled = false
	conf.Proxy.Enabled = false
	conf.SSH.Enabled = true

	require.NoError(t, node.CreateEx(t, nil, conf))

	t.Cleanup(func() {
		node.StopAll()
	})
	require.NoError(t, node.Start())

	if p.node == nil {
		p.node = node
	} else {
		t.Error("node already initialized")
	}
}
