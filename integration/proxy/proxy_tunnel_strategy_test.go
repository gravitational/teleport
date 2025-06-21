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

package proxy

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/cloud/imds"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

type proxyTunnelStrategy struct {
	username string
	cluster  string
	strategy *types.TunnelStrategyV1

	lb      *utils.LoadBalancer
	auth    *helpers.TeleInstance
	proxies []*helpers.TeleInstance
	node    *helpers.TeleInstance

	db           *helpers.TeleInstance
	dbAuthClient *authclient.Client
	postgresDB   *postgres.TestServer
}

func newProxyTunnelStrategy(t *testing.T, cluster string, strategy *types.TunnelStrategyV1) *proxyTunnelStrategy {
	p := &proxyTunnelStrategy{
		cluster:  cluster,
		username: helpers.MustGetCurrentUser(t).Username,
		strategy: strategy,
	}

	lib.SetInsecureDevMode(true)
	t.Cleanup(func() {
		lib.SetInsecureDevMode(false)
		p.cleanup(t)
	})

	return p
}

// testProxyTunnelStrategyAgentMesh tests the agent-mesh tunnel strategy
func TestProxyTunnelStrategyAgentMesh(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		testResource func(*testing.T, *proxyTunnelStrategy)
	}{
		{
			name: "SSHAccess",
			testResource: func(t *testing.T, p *proxyTunnelStrategy) {
				// bootstrap a node instance.
				p.makeNode(t)

				// wait for the node to be connected to both proxies
				helpers.WaitForActiveTunnelConnections(t, p.proxies[0].Tunnel, p.cluster, 1)
				helpers.WaitForActiveTunnelConnections(t, p.proxies[1].Tunnel, p.cluster, 1)

				// make sure we can connect to the node going through any proxy.
				p.waitForNodeToBeReachable(t)
				p.dialNode(t)
			},
		},
		{
			name: "DatabaseAccess",
			testResource: func(t *testing.T, p *proxyTunnelStrategy) {
				p.makeDatabase(t)

				// wait for the database to be connected to both proxies
				helpers.WaitForActiveTunnelConnections(t, p.proxies[0].Tunnel, p.cluster, 1)
				helpers.WaitForActiveTunnelConnections(t, p.proxies[1].Tunnel, p.cluster, 1)

				// make sure we can connect to the database going through any proxy.
				p.waitForDatabaseToBeReachable(t)
				p.dialDatabase(t)
			},
		},
	}

	for _, tc := range tests {
		// capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := newProxyTunnelStrategy(t, "proxy-tunnel-agent-mesh",
				&types.TunnelStrategyV1{
					Strategy: &types.TunnelStrategyV1_AgentMesh{
						AgentMesh: types.DefaultAgentMeshTunnelStrategy(),
					},
				},
			)

			// bootstrap a load balancer for proxies.
			p.makeLoadBalancer(t)

			// bootstrap an auth instance.
			p.makeAuth(t)

			// bootstrap two proxy instances.
			p.makeProxy(t)
			p.makeProxy(t)
			require.Len(t, p.proxies, 2)

			tc.testResource(t, p)
		})
	}
}

// TestProxyTunnelStrategyProxyPeering tests the proxy-peer tunnel strategy.
func TestProxyTunnelStrategyProxyPeering(t *testing.T) {
	// TODO(jakule): Fix the test.
	t.Skip("this test is flaky as it very sensitive to our timeouts")

	// This test cannot run in parallel as set module changes the global state.
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DB: {Enabled: true},
			},
		},
	})

	p := newProxyTunnelStrategy(t, "proxy-tunnel-proxy-peer",
		&types.TunnelStrategyV1{
			Strategy: &types.TunnelStrategyV1_ProxyPeering{
				ProxyPeering: types.DefaultProxyPeeringTunnelStrategy(),
			},
		},
	)

	// bootstrap a load balancer for proxies.
	p.makeLoadBalancer(t)

	// bootstrap an auth instance.
	p.makeAuth(t)

	// bootstrap the first proxy instance.
	p.makeProxy(t)
	require.Len(t, p.proxies, 1)

	// bootstrap a node instance.
	p.makeNode(t)

	// bootstrap a db instance.
	p.makeDatabase(t)

	// wait for the node and db to open reverse tunnels to the first proxy.
	helpers.WaitForActiveTunnelConnections(t, p.proxies[0].Tunnel, p.cluster, 2)

	// bootstrap the second proxy instance after the node and db have already
	// established reverse tunnels to the first proxy.
	p.makeProxy(t)
	require.Len(t, p.proxies, 2)

	// make sure both proxies are connected to each other.
	waitForActivePeerProxyConnections(t, p.proxies[0].Tunnel, 1)
	waitForActivePeerProxyConnections(t, p.proxies[1].Tunnel, 1)

	// make sure we can connect to the node going through any proxy.
	p.waitForNodeToBeReachable(t)
	p.dialNode(t)

	// make sure we can connect to the database going through any proxy.
	p.waitForDatabaseToBeReachable(t)
	p.dialDatabase(t)
}

// dialNode starts a client conn to a node reachable through a specific proxy.
func (p *proxyTunnelStrategy) dialNode(t *testing.T) {
	for _, proxy := range p.proxies {
		creds, err := helpers.GenerateUserCreds(helpers.UserCredsRequest{
			Process:  p.auth.Process,
			Username: p.username,
		})
		require.NoError(t, err)

		client, err := proxy.NewClientWithCreds(
			helpers.ClientConfig{
				Cluster: p.cluster,
				Host:    p.node.Process.Config.HostUUID,
			},
			*creds,
		)
		require.NoError(t, err)

		output := &bytes.Buffer{}
		client.Stdout = output

		cmd := []string{"echo", "hello world"}
		err = client.SSH(context.Background(), cmd)
		require.NoError(t, err)
		require.Equal(t, "hello world\n", output.String())
	}
}

func (p *proxyTunnelStrategy) dialDatabase(t *testing.T) {
	for i, proxy := range p.proxies {
		connClient, err := postgres.MakeTestClient(context.Background(), common.TestClientConfig{
			AuthClient: p.dbAuthClient,
			AuthServer: p.auth.Process.GetAuthServer(),
			Address:    proxy.Web,
			Cluster:    p.cluster,
			Username:   p.username,
			RouteToDatabase: tlsca.RouteToDatabase{
				ServiceName: p.cluster + "-postgres",
				Protocol:    defaults.ProtocolPostgres,
				Username:    "postgres",
				Database:    "test",
			},
		})
		require.NoError(t, err)

		result, err := connClient.Exec(context.Background(), "select 1").ReadAll()
		require.NoError(t, err)
		require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, result)
		require.Equal(t, uint32(i+1), p.postgresDB.QueryCount())

		err = connClient.Close(context.Background())
		require.NoError(t, err)
	}
}

// makeLoadBalancer bootsraps a new load balancer for proxy instances.
func (p *proxyTunnelStrategy) makeLoadBalancer(t *testing.T) {
	if p.lb != nil {
		require.Fail(t, "load balancer already initialized")
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	lbAddr := utils.MustParseAddr(net.JoinHostPort(helpers.Loopback, "0"))
	lb, err := utils.NewLoadBalancer(ctx, *lbAddr)
	require.NoError(t, err)

	require.NoError(t, lb.Listen())
	t.Cleanup(func() {
		require.NoError(t, lb.Close())
	})
	go lb.Serve()

	p.lb = lb
}

// makeAuth bootsraps a new teleport auth instance.
func (p *proxyTunnelStrategy) makeAuth(t *testing.T) {
	if p.auth != nil {
		require.Fail(t, "auth already initialized")
	}

	privateKey, publicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	auth := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: p.cluster,
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Priv:        privateKey,
		Pub:         publicKey,
		Logger:      utils.NewSlogLoggerForTests(),
	})

	auth.AddUser(p.username, []string{p.username})

	conf := servicecfg.MakeDefaultConfig()
	conf.DataDir = t.TempDir()
	conf.Logger = auth.Log

	conf.Auth.Enabled = true
	conf.Auth.NetworkingConfig.SetTunnelStrategy(p.strategy)
	conf.Auth.SessionRecordingConfig.SetMode(types.RecordAtNodeSync)
	conf.Proxy.Enabled = false
	conf.SSH.Enabled = false

	require.NoError(t, auth.CreateEx(t, nil, conf))
	require.NoError(t, auth.Start())

	p.auth = auth
}

// makeProxy bootstraps a new teleport proxy instance.
// Its public address points to a load balancer.
func (p *proxyTunnelStrategy) makeProxy(t *testing.T) {
	proxy := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: p.cluster,
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Logger:      utils.NewSlogLoggerForTests(),
	})

	authAddr := utils.MustParseAddr(p.auth.Auth)

	conf := servicecfg.MakeDefaultConfig()
	conf.SetAuthServerAddress(*authAddr)
	conf.SetToken("token")
	conf.DataDir = t.TempDir()
	conf.Logger = proxy.Log
	conf.InstanceMetadataClient = imds.NewDisabledIMDSClient()

	conf.Auth.Enabled = false
	conf.SSH.Enabled = false

	conf.Proxy.Enabled = true
	conf.Proxy.ReverseTunnelListenAddr.Addr = proxy.ReverseTunnel
	conf.Proxy.SSHAddr.Addr = proxy.SSHProxy
	conf.Proxy.WebAddr.Addr = proxy.Web
	conf.Proxy.PeerAddress.Addr = helpers.NewListenerOn(t, helpers.Loopback, service.ListenerProxyPeer, &proxy.Fds)
	conf.Proxy.PeerPublicAddr = conf.Proxy.PeerAddress
	conf.Proxy.PublicAddrs = append(conf.Proxy.PublicAddrs, utils.FromAddr(p.lb.Addr()))
	conf.Proxy.DisableWebInterface = true
	conf.FileDescriptors = proxy.Fds

	process, err := service.NewTeleport(conf)
	require.NoError(t, err)
	proxy.Config = conf
	proxy.Process = process

	p.lb.AddBackend(conf.Proxy.WebAddr)
	require.NoError(t, proxy.Start())

	p.proxies = append(p.proxies, proxy)
}

// makeNode bootstraps a new teleport node instance.
// It connects to a proxy via a reverse tunnel going through a load balancer.
func (p *proxyTunnelStrategy) makeNode(t *testing.T) {
	if p.node != nil {
		require.Fail(t, "node already initialized")
	}

	node := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: p.cluster,
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Logger:      utils.NewSlogLoggerForTests(),
	})

	conf := servicecfg.MakeDefaultConfig()
	conf.Version = types.V3
	conf.SetToken("token")
	conf.DataDir = t.TempDir()
	conf.Logger = node.Log
	conf.InstanceMetadataClient = imds.NewDisabledIMDSClient()

	conf.Auth.Enabled = false
	conf.Proxy.Enabled = false
	conf.SSH.Enabled = true
	conf.ProxyServer = utils.FromAddr(p.lb.Addr())

	process, err := service.NewTeleport(conf)
	require.NoError(t, err)
	node.Config = conf
	node.Process = process

	require.NoError(t, node.Start())

	p.node = node
}

// makeDatabase bootstraps a new teleport db instance.
// It connects to a proxy via a reverse tunnel going through a load balancer.
func (p *proxyTunnelStrategy) makeDatabase(t *testing.T) {
	if p.db != nil {
		require.Fail(t, "database already initialized")
	}

	dbListener, err := net.Listen("tcp", net.JoinHostPort(helpers.Host, "0"))
	require.NoError(t, err)

	_, portStr, err := net.SplitHostPort(dbListener.Addr().String())
	require.NoError(t, err)

	dbAddr := net.JoinHostPort(helpers.Host, portStr)

	// setup database service
	db := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: p.cluster,
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Logger:      utils.NewSlogLoggerForTests(),
	})

	conf := servicecfg.MakeDefaultConfig()
	conf.Version = types.V3
	conf.SetToken("token")
	conf.DataDir = t.TempDir()
	conf.Logger = db.Log
	conf.InstanceMetadataClient = imds.NewDisabledIMDSClient()

	conf.Auth.Enabled = false
	conf.Proxy.Enabled = false
	conf.SSH.Enabled = false
	conf.Databases.Enabled = true
	conf.ProxyServer = utils.FromAddr(p.lb.Addr())
	conf.Databases.Databases = []servicecfg.Database{
		{
			Name:     p.cluster + "-postgres",
			Protocol: defaults.ProtocolPostgres,
			URI:      dbAddr,
		},
	}

	_, role, err := auth.CreateUserAndRole(p.auth.Process.GetAuthServer(), p.username, []string{p.username}, nil)
	require.NoError(t, err)

	role.SetDatabaseUsers(types.Allow, []string{types.Wildcard})
	role.SetDatabaseNames(types.Allow, []string{types.Wildcard})
	_, err = p.auth.Process.GetAuthServer().UpsertRole(context.Background(), role)
	require.NoError(t, err)

	// start the process and block until specified events are received.
	process, err := service.NewTeleport(conf)
	require.NoError(t, err)
	db.Config = conf
	db.Process = process

	receivedEvents, err := helpers.StartAndWait(db.Process, []string{
		service.DatabasesIdentityEvent,
		service.DatabasesReady,
		service.TeleportReadyEvent,
	})
	require.NoError(t, err)

	var client *authclient.Client
	for _, event := range receivedEvents {
		if event.Name == service.DatabasesIdentityEvent {
			conn, ok := (event.Payload).(*service.Connector)
			require.True(t, ok)
			client = conn.Client
			break
		}
	}
	require.NotNil(t, client)

	// setup a test postgres database
	postgresDB, err := postgres.NewTestServer(common.TestServerConfig{
		AuthClient: client,
		Name:       p.cluster + "-postgres",
		Listener:   dbListener,
	})
	require.NoError(t, err)
	go postgresDB.Serve()

	p.db = db
	p.dbAuthClient = client
	p.postgresDB = postgresDB
}

// waitForNodeToBeReachable waits for the node to be reachable from all
// proxies by making sure the proxy peer connectivity info (if any) got
// propagated to the auth server.
func (p *proxyTunnelStrategy) waitForNodeToBeReachable(t *testing.T) {
	check := func(t *helpers.TeleInstance, availability int) (bool, error) {
		nodes, err := t.GetSiteAPI(p.cluster).GetNodes(
			context.Background(),
			apidefaults.Namespace,
		)
		if err != nil {
			return false, trace.Wrap(err)
		}

		for _, node := range nodes {
			if node.GetName() == p.node.Process.Config.HostUUID &&
				len(node.GetProxyIDs()) == availability {
				return true, nil
			}
		}
		return false, nil
	}
	p.waitForResource(t, string(types.RoleNode), check)
}

// waitForDatabaseToBeReachable waits for the database to be reachable from all
// proxies by making sure the proxy peer connectivity info (if any) got
// propagated to the auth server.
func (p *proxyTunnelStrategy) waitForDatabaseToBeReachable(t *testing.T) {
	check := func(t *helpers.TeleInstance, availability int) (bool, error) {
		databases, err := t.GetSiteAPI(p.cluster).GetDatabaseServers(
			context.Background(),
			apidefaults.Namespace,
		)
		if err != nil {
			return false, trace.Wrap(err)
		}

		for _, db := range databases {
			if db.GetHostID() == p.db.Process.Config.HostUUID &&
				len(db.GetProxyIDs()) == availability {
				return true, nil
			}
		}

		return false, nil
	}
	p.waitForResource(t, string(types.RoleDatabase), check)
}

// waitForResource waits for each proxy to satisfy the check function defined as a parameter
// in a certain defined timeframe.
func (p *proxyTunnelStrategy) waitForResource(t *testing.T, role string, check func(*helpers.TeleInstance, int) (bool, error)) {
	availability := 0
	if proxyPeeringStrategy := p.strategy.GetProxyPeering(); proxyPeeringStrategy != nil {
		availability = int(proxyPeeringStrategy.AgentConnectionCount)
	}

	require.Eventually(t, func() bool {
		propagated := 0
		for _, proxy := range p.proxies {
			ok, err := check(proxy, availability)
			if err != nil {
				return false
			}
			if !ok {
				return false
			}
			propagated++
		}

		return len(p.proxies) == propagated
	},
		30*time.Second,
		time.Second,
		"Resource %s was not available %v in the expected time frame", role, 30*time.Second,
	)
}

// cleanup stops all resources started during tests
func (p *proxyTunnelStrategy) cleanup(t *testing.T) {
	var errs []error

	if p.postgresDB != nil {
		errs = append(errs, p.postgresDB.Close())
	}

	if p.db != nil {
		errs = append(errs, p.db.StopAll())
	}

	if p.node != nil {
		errs = append(errs, p.node.StopAll())
	}

	for _, proxy := range p.proxies {
		if proxy != nil {
			errs = append(errs, proxy.StopAll())
		}
	}

	if p.auth != nil {
		errs = append(errs, p.auth.StopAll())
	}

	if p.lb != nil {
		errs = append(errs, p.lb.Close())
	}

	require.NoError(t, trace.NewAggregate(errs...))
}
