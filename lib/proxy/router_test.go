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
	"math/rand/v2"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/agentless"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services/readonly"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
)

type testSite struct {
	cfg        types.ClusterNetworkingConfig
	nodes      []types.Server
	gitServers []types.Server
}

func (t testSite) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	return t.cfg, nil
}

func (t testSite) GetNodes(ctx context.Context, fn func(n readonly.Server) bool) ([]types.Server, error) {
	var out []types.Server
	for _, s := range t.nodes {
		if fn(s) {
			out = append(out, s)
		}
	}

	return out, nil
}
func (t testSite) GetGitServers(ctx context.Context, fn func(n readonly.Server) bool) ([]types.Server, error) {
	var out []types.Server
	for _, s := range t.gitServers {
		if fn(s) {
			out = append(out, s)
		}
	}

	return out, nil
}

type server struct {
	name     string
	hostname string
	addr     string
	tunnel   bool
}

func createServers(srvs []server) []types.Server {
	out := make([]types.Server, 0, len(srvs))
	for _, s := range srvs {
		srv := &types.ServerV2{
			Kind:    types.KindNode,
			Version: types.V2,
			Metadata: types.Metadata{
				Name: s.name,
			},
			Spec: types.ServerSpecV2{
				Addr:      s.addr,
				Hostname:  s.hostname,
				UseTunnel: s.tunnel,
			},
		}
		out = append(out, srv)
	}

	return out
}

type mockHostResolver struct {
	hosts map[string][]string
}

func (r *mockHostResolver) LookupHost(ctx context.Context, host string) (addrs []string, err error) {
	return r.hosts[host], nil
}

// TestRouteScoring verifies expected behavior in the specific cases where multiple matches
// of different quality are made.
func TestRouteScoring(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// set up various servers with overlapping IPs and hostnames
	servers := createServers([]server{
		{
			name:     uuid.NewString(),
			hostname: "one.example.com",
			addr:     "1.2.3.4:123",
		},
		{
			name:     uuid.NewString(),
			hostname: "two.example.com",
			addr:     "1.2.3.4:456",
		},
		{
			name:     uuid.NewString(),
			hostname: "dupe.example.com",
			addr:     "1.2.3.4:789",
		},
		{
			name:     uuid.NewString(),
			hostname: "dupe.example.com",
			addr:     "1.2.3.4:1011",
		},
		{
			name:     uuid.NewString(),
			hostname: "blue.example.com",
			addr:     "2.3.4.5:22",
		},
		{
			name:     "not-a-uuid",
			hostname: "test.example.com",
			addr:     "3.4.5.6:22",
		},
	})

	// scoring behavior is independent of routing strategy so we just
	// use the most strict config for all cases.
	site := &testSite{
		cfg: &types.ClusterNetworkingConfigV2{
			Spec: types.ClusterNetworkingConfigSpecV2{
				RoutingStrategy: types.RoutingStrategy_UNAMBIGUOUS_MATCH,
			},
		},
		nodes: servers,
	}

	// set up resolver
	resolver := &mockHostResolver{
		hosts: map[string][]string{
			// register a hostname that only indirectly maps to a node
			"red.example.com": []string{"2.3.4.5"},
		},
	}

	for _, s := range servers {
		resolver.hosts[s.GetHostname()] = []string{"1.2.3.4"}
	}

	tts := []struct {
		desc       string
		host, port string
		expect     string
		ambiguous  bool
	}{
		{
			// this is the primary case that route scoring was implemented to solve. prior to scoring,
			// dialing by a hostname that is itself unambiguous but resolves to an ip that
			// *is* ambiguous would result in an unexpected ambiguous host error, despite the fact that
			// what the user typed in was clearly unambiguous.
			desc:   "dial by hostname",
			host:   "one.example.com",
			expect: "one.example.com",
		},
		{
			desc:   "dial by ip only",
			host:   "2.3.4.5",
			expect: "blue.example.com",
		},
		{
			desc:   "dial by ip and port",
			host:   "1.2.3.4",
			port:   "456",
			expect: "two.example.com",
		},
		{
			desc:      "ambiguous hostname dial",
			host:      "dupe.example.com",
			ambiguous: true,
		},
		{
			desc:      "ambiguous ip dial",
			host:      "1.2.3.4",
			ambiguous: true,
		},
		{
			desc:   "disambiguate by port",
			host:   "dupe.example.com",
			port:   "789",
			expect: "dupe.example.com",
		},
		{
			desc:   "indirect ip resolve",
			host:   "red.example.com",
			expect: "blue.example.com",
		},
		{
			desc:   "non-uuid name",
			host:   "not-a-uuid",
			expect: "test.example.com",
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			srv, err := getServerWithResolver(ctx, tt.host, tt.port, site, resolver)
			if tt.ambiguous {
				require.ErrorIs(t, err, teleport.ErrNodeIsAmbiguous)
				return
			}
			require.Equal(t, tt.expect, srv.GetHostname())
		})
	}
}

func TestGetServers(t *testing.T) {
	t.Parallel()

	mostRecentCfg := types.ClusterNetworkingConfigV2{
		Spec: types.ClusterNetworkingConfigSpecV2{
			RoutingStrategy: types.RoutingStrategy_MOST_RECENT,
		},
	}

	unambiguousCfg := types.ClusterNetworkingConfigV2{
		Spec: types.ClusterNetworkingConfigSpecV2{
			RoutingStrategy: types.RoutingStrategy_UNAMBIGUOUS_MATCH,
		},
	}

	unambiguousInsensitiveCfg := types.ClusterNetworkingConfigV2{
		Spec: types.ClusterNetworkingConfigSpecV2{
			RoutingStrategy:        types.RoutingStrategy_UNAMBIGUOUS_MATCH,
			CaseInsensitiveRouting: true,
		},
	}

	hostID := uuid.NewString()
	const ec2ID = "012345678901-i-01234567890abcdef"

	servers := createServers([]server{
		{
			name:     hostID,
			hostname: "llama",
			addr:     "llama:123",
		},
		{
			name:     "llama",
			hostname: "llama",
			addr:     "llama:123",
			tunnel:   true,
		},
		{
			name:     "llama",
			hostname: hostID,
			addr:     "llama:123",
		},
		{
			name:     ec2ID,
			hostname: "node.aws",
			addr:     "node.aws:123",
		},
		{
			name:     "node.aws",
			hostname: "node.aws",
			addr:     "node.aws:123",
			tunnel:   true,
		},
		{
			name:     "node.aws",
			hostname: ec2ID,
			addr:     "node.aws:123",
		},
		{
			name:     "alpaca",
			hostname: "alpaca",
			addr:     "alpaca:123",
			tunnel:   true,
		},
		{
			name:     "alpaca",
			hostname: "localhost",
			addr:     "alpaca:987",
			tunnel:   true,
		},
		{
			name:     "goat",
			hostname: "goat",
			addr:     "1.2.3.4:123",
		},
		{
			name:     "sheep",
			hostname: "sheep",
			addr:     "sheep.bah:0",
		},
		{
			name:     "sheep2",
			hostname: "sheep",
			addr:     "sheep.bah:0",
		},
		{
			name:     "lion",
			hostname: "lion",
			addr:     "lion.roar",
		},
		{
			name:     "platypus1",
			hostname: "Platypus",
			tunnel:   true,
		},
		{
			name:     "platypus2",
			hostname: "platypus",
			tunnel:   true,
		},
		{
			name:     "capybara1",
			hostname: "Capybara",
			tunnel:   true,
		},
	})

	servers = append(servers,
		&types.ServerV2{
			Kind:    types.KindNode,
			SubKind: types.SubKindOpenSSHNode,
			Version: types.V2,
			Metadata: types.Metadata{
				Name: "agentless-node-1",
			},
			Spec: types.ServerSpecV2{
				Addr:     "1.2.3.4:22",
				Hostname: "agentless-1",
			},
		},
	)

	gitServers := []types.Server{
		makeGitHubServer(t, "org1"),
		makeGitHubServer(t, "org2"),
	}

	// ensure tests don't have order-dependence
	rand.Shuffle(len(servers), func(i, j int) {
		servers[i], servers[j] = servers[j], servers[i]
	})

	cases := []struct {
		name            string
		host            string
		port            string
		site            testSite
		errAssertion    require.ErrorAssertionFunc
		serverAssertion func(t *testing.T, srv types.Server)
	}{
		{
			name:         "no matches for hostname",
			site:         testSite{cfg: &unambiguousCfg},
			host:         "test",
			errAssertion: require.NoError,
			serverAssertion: func(t *testing.T, srv types.Server) {
				require.Empty(t, srv)
			},
		},
		{
			name: "no matches for uuid",
			site: testSite{cfg: &mostRecentCfg},
			host: uuid.NewString(),
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err), i...)
			},
			serverAssertion: func(t *testing.T, srv types.Server) {
				require.Empty(t, srv)
			},
		},
		{
			name: "no matches for ec2 id",
			site: testSite{cfg: &unambiguousCfg},
			host: "123456789012-i-1234567890abcdef0",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err), i...)
			},
			serverAssertion: func(t *testing.T, srv types.Server) {
				require.Empty(t, srv)
			},
		},
		{
			name: "ambiguous match fails",
			site: testSite{cfg: &unambiguousCfg, nodes: servers},
			host: "sheep",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, teleport.ErrNodeIsAmbiguous)
			},
			serverAssertion: func(t *testing.T, srv types.Server) {
				require.Empty(t, srv)
			},
		},
		{
			name:         "ambiguous match returns most recent",
			site:         testSite{cfg: &mostRecentCfg, nodes: servers},
			host:         "sheep",
			errAssertion: require.NoError,
			serverAssertion: func(t *testing.T, srv types.Server) {
				require.NotNil(t, srv)
				require.Equal(t, "sheep", srv.GetHostname())
			},
		},
		{
			name:         "match by uuid",
			site:         testSite{cfg: &unambiguousCfg, nodes: servers},
			host:         hostID,
			errAssertion: require.NoError,
			serverAssertion: func(t *testing.T, srv types.Server) {
				require.NotNil(t, srv)
				require.Equal(t, "llama", srv.GetHostname())
			},
		},
		{
			name:         "match by ec2 id",
			site:         testSite{cfg: &unambiguousCfg, nodes: servers},
			host:         ec2ID,
			errAssertion: require.NoError,
			serverAssertion: func(t *testing.T, srv types.Server) {
				require.NotNil(t, srv)
				require.Equal(t, "node.aws", srv.GetHostname())
			},
		},
		{
			name:         "match by ip",
			site:         testSite{cfg: &unambiguousCfg, nodes: servers},
			host:         "1.2.3.4",
			port:         "123",
			errAssertion: require.NoError,
			serverAssertion: func(t *testing.T, srv types.Server) {
				require.NotNil(t, srv)
				require.Equal(t, "goat", srv.GetHostname())
			},
		},
		{
			name:         "match by host only for tunnels",
			site:         testSite{cfg: &unambiguousCfg, nodes: servers},
			host:         "alpaca",
			errAssertion: require.NoError,
			serverAssertion: func(t *testing.T, srv types.Server) {
				require.NotNil(t, srv)
				require.Equal(t, "alpaca", srv.GetHostname())
			},
		},
		{
			name:         "case-insensitive match",
			site:         testSite{cfg: &unambiguousInsensitiveCfg, nodes: servers},
			host:         "capybara",
			errAssertion: require.NoError,
			serverAssertion: func(t *testing.T, srv types.Server) {
				require.NotNil(t, srv)
				require.Equal(t, "Capybara", srv.GetHostname())
			},
		},
		{
			name: "case-insensitive ambiguous",
			site: testSite{cfg: &unambiguousInsensitiveCfg, nodes: servers},
			host: "platypus",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, teleport.ErrNodeIsAmbiguous)
			},
			serverAssertion: func(t *testing.T, srv types.Server) {
				require.Empty(t, srv)
			},
		},
		{
			name:         "agentless match by non-uuid name",
			site:         testSite{cfg: &unambiguousCfg, nodes: servers},
			host:         "agentless-node-1",
			errAssertion: require.NoError,
			serverAssertion: func(t *testing.T, srv types.Server) {
				require.NotNil(t, srv)
				require.Equal(t, "agentless-1", srv.GetHostname())
				require.True(t, srv.IsOpenSSHNode())
			},
		},
		{
			name:         "git server",
			site:         testSite{cfg: &unambiguousCfg, gitServers: gitServers},
			host:         "org2.teleport-github-org",
			errAssertion: require.NoError,
			serverAssertion: func(t *testing.T, srv types.Server) {
				require.NotNil(t, srv)
				require.NotNil(t, srv.GetGitHub())
				assert.Equal(t, "org2", srv.GetGitHub().Organization)
			},
		},
		{
			name: "git server not found",
			site: testSite{cfg: &unambiguousCfg, gitServers: gitServers},
			host: "org-not-found.teleport-github-org",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err), i...)
			},
			serverAssertion: func(t *testing.T, srv types.Server) {
				require.Nil(t, srv)
			},
		},
	}

	ctx := context.Background()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := getServer(ctx, tt.host, tt.port, tt.site)
			tt.errAssertion(t, err)
			tt.serverAssertion(t, srv)
		})
	}
}

func serverResolver(srv types.Server, err error) serverResolverFn {
	return func(ctx context.Context, host, port string, site site) (types.Server, error) {
		return srv, err
	}
}

type mockConn struct {
	net.Conn
	buff bytes.Buffer
}

func (o *mockConn) Read(p []byte) (n int, err error) {
	return o.buff.Read(p)
}

func (o *mockConn) Write(p []byte) (n int, err error) {
	return o.buff.Write(p)
}

func (o *mockConn) Close() error {
	return nil
}

func TestCheckedPrefixWriter(t *testing.T) {
	t.Parallel()
	testData := []byte("test data")
	t.Run("missing prefix", func(t *testing.T) {
		t.Run("single write", func(t *testing.T) {
			cpw := newCheckedPrefixWriter(&mockConn{}, []byte("wrong"))

			_, err := cpw.Write(testData)
			require.True(t, trace.IsAccessDenied(err), "expected trace.AccessDenied error, got: %v", err)
		})
		t.Run("two writes", func(t *testing.T) {
			cpw := newCheckedPrefixWriter(&mockConn{}, append(testData, []byte("wrong")...))

			_, err := cpw.Write(testData)
			require.NoError(t, err)

			_, err = cpw.Write(testData)
			require.True(t, trace.IsAccessDenied(err), "expected trace.AccessDenied error, got: %v", err)
		})
	})
	t.Run("success", func(t *testing.T) {
		t.Run("single write", func(t *testing.T) {
			cpw := newCheckedPrefixWriter(&mockConn{}, []byte("test"))

			// First write with correct prefix should be successful
			_, err := cpw.Write(testData)
			require.NoError(t, err)

			// Write some additional data
			secondData := []byte("second data")
			_, err = cpw.Write(secondData)
			require.NoError(t, err)

			// Resulting read should contain data from both writes
			buf := make([]byte, len(testData)+len(secondData))
			_, err = cpw.Read(buf)
			require.NoError(t, err)
			require.Equal(t, append(testData, secondData...), buf)
		})
		t.Run("two writes", func(t *testing.T) {
			cpw := newCheckedPrefixWriter(&mockConn{}, []byte("test"))

			// First write gives part of correct prefix
			_, err := cpw.Write(testData[:3])
			require.NoError(t, err)

			// Second write gives the rest of correct prefix
			_, err = cpw.Write(testData[3:])
			require.NoError(t, err)

			// Write some additional data
			secondData := []byte("second data")
			_, err = cpw.Write(secondData)
			require.NoError(t, err)

			// Resulting read should contain all written data
			buf := make([]byte, len(testData)+len(secondData))
			_, err = cpw.Read(buf)
			require.NoError(t, err)
			require.Equal(t, append(testData, secondData...), buf)
		})
	})
}

type tunnel struct {
	reversetunnelclient.Tunnel

	site reversetunnelclient.RemoteSite
	err  error
}

func (t tunnel) GetSite(cluster string) (reversetunnelclient.RemoteSite, error) {
	return t.site, t.err
}

type testRemoteSite struct {
	reversetunnelclient.RemoteSite

	params reversetunnelclient.DialParams

	conn net.Conn
	err  error
}

func (r *testRemoteSite) Dial(params reversetunnelclient.DialParams) (net.Conn, error) {
	r.params = params
	return r.conn, r.err
}

func (r testRemoteSite) DialAuthServer(reversetunnelclient.DialParams) (net.Conn, error) {
	return r.conn, r.err
}

func (r testRemoteSite) GetClient() (authclient.ClientI, error) {
	return nil, nil
}

type testSiteGetter struct {
	site reversetunnelclient.RemoteSite
}

func (s testSiteGetter) GetSite(clusterName string) (reversetunnelclient.RemoteSite, error) {
	return s.site, nil
}

type fakeConn struct {
	net.Conn
}

func TestRouter_DialHost(t *testing.T) {
	t.Parallel()

	srv := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: uuid.NewString(),
		},
		Spec: types.ServerSpecV2{
			Addr:     "127.0.0.1:8889",
			Hostname: "test",
		},
	}
	agentlessSrv := &types.ServerV2{
		Kind:    types.KindNode,
		SubKind: types.SubKindOpenSSHNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: "agentless",
		},
		Spec: types.ServerSpecV2{
			Addr:     "127.0.0.1:9001",
			Hostname: "agentless",
		},
	}

	agentlessEC2ICESrv := &types.ServerV2{
		Kind:    types.KindNode,
		SubKind: types.SubKindOpenSSHEICENode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: uuid.NewString(),
		},
		Spec: types.ServerSpecV2{
			Addr:     "127.0.0.1:9001",
			Hostname: "agentless",
		},
	}

	agentGetter := func() (teleagent.Agent, error) {
		return nil, nil
	}
	createSigner := func(_ context.Context, _ agentless.LocalAccessPoint, _ agentless.CertGenerator) (ssh.Signer, error) {
		key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
		if err != nil {
			return nil, err
		}
		return ssh.NewSignerFromSigner(key)
	}

	cases := []struct {
		name      string
		router    Router
		assertion func(t *testing.T, params reversetunnelclient.DialParams, conn net.Conn, err error)
	}{
		{
			name: "failure resolving node",
			router: Router{
				clusterName:    "test",
				tracer:         tracing.NoopTracer("test"),
				serverResolver: serverResolver(nil, teleport.ErrNodeIsAmbiguous),
			},
			assertion: func(t *testing.T, params reversetunnelclient.DialParams, conn net.Conn, err error) {
				require.Error(t, err)
				require.Nil(t, conn)
			},
		},
		{
			name: "failure looking up cluster",
			router: Router{
				clusterName: "leaf",
				siteGetter:  tunnel{err: trace.NotFound("unknown cluster")},
				tracer:      tracing.NoopTracer("test"),
			},
			assertion: func(t *testing.T, params reversetunnelclient.DialParams, conn net.Conn, err error) {
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
				require.Nil(t, conn)
			},
		},
		{
			name: "dial failure",
			router: Router{
				clusterName:    "test",
				localSite:      &testRemoteSite{err: trace.ConnectionProblem(context.DeadlineExceeded, "connection refused")},
				tracer:         tracing.NoopTracer("test"),
				serverResolver: serverResolver(srv, nil),
			},
			assertion: func(t *testing.T, params reversetunnelclient.DialParams, conn net.Conn, err error) {
				require.Error(t, err)
				require.True(t, trace.IsConnectionProblem(err))
				require.Nil(t, conn)
			},
		},
		{
			name: "dial success",
			router: Router{
				clusterName:    "test",
				localSite:      &testRemoteSite{conn: fakeConn{}},
				tracer:         tracing.NoopTracer("test"),
				serverResolver: serverResolver(srv, nil),
			},
			assertion: func(t *testing.T, params reversetunnelclient.DialParams, conn net.Conn, err error) {
				require.NoError(t, err)
				require.Equal(t, srv, params.TargetServer)
				require.NotNil(t, params.GetUserAgent)
				require.Nil(t, params.AgentlessSigner)
				require.NotNil(t, conn)
				require.Contains(t, params.Principals, "host")
				require.Contains(t, params.Principals, "host.test")
			},
		},
		{
			name: "dial success to agentless node",
			router: Router{
				clusterName:    "test",
				localSite:      &testRemoteSite{conn: fakeConn{}},
				siteGetter:     &testSiteGetter{site: &testRemoteSite{conn: fakeConn{}}},
				tracer:         tracing.NoopTracer("test"),
				serverResolver: serverResolver(agentlessSrv, nil),
			},
			assertion: func(t *testing.T, params reversetunnelclient.DialParams, conn net.Conn, err error) {
				require.NoError(t, err)
				require.Equal(t, agentlessSrv, params.TargetServer)
				require.Nil(t, params.GetUserAgent)
				require.NotNil(t, params.AgentlessSigner)
				require.True(t, params.IsAgentlessNode)
				require.NotNil(t, conn)
				require.Contains(t, params.Principals, "host")
				require.Contains(t, params.Principals, "host.test")
			},
		},
		{
			name: "dial success to agentless node using EC2 Instance Connect Endpoint",
			router: Router{
				clusterName:    "test",
				localSite:      &testRemoteSite{conn: fakeConn{}},
				siteGetter:     &testSiteGetter{site: &testRemoteSite{conn: fakeConn{}}},
				tracer:         tracing.NoopTracer("test"),
				serverResolver: serverResolver(agentlessEC2ICESrv, nil),
			},
			assertion: func(t *testing.T, params reversetunnelclient.DialParams, conn net.Conn, err error) {
				require.NoError(t, err)
				require.Equal(t, agentlessEC2ICESrv, params.TargetServer)
				require.Nil(t, params.GetUserAgent)
				require.Nil(t, params.AgentlessSigner)
				require.True(t, params.IsAgentlessNode)
				require.NotNil(t, conn)
			},
		},
	}

	ctx := context.Background()

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := tt.router.DialHost(ctx, &utils.NetAddr{}, &utils.NetAddr{}, "host", "0", "test", nil, agentGetter, createSigner)

			var params reversetunnelclient.DialParams
			if tt.router.localSite != nil {
				params = tt.router.localSite.(*testRemoteSite).params
			}

			tt.assertion(t, params, conn, err)
		})
	}
}

func TestRouter_DialSite(t *testing.T) {
	t.Parallel()

	const cluster = "test"

	cases := []struct {
		name      string
		cluster   string
		localSite testRemoteSite
		tunnel    tunnel
		assertion func(t *testing.T, conn net.Conn, err error)
	}{
		{
			name:      "failure to dial local site",
			cluster:   cluster,
			localSite: testRemoteSite{err: trace.ConnectionProblem(context.DeadlineExceeded, "connection refused")},
			assertion: func(t *testing.T, conn net.Conn, err error) {
				require.Error(t, err)
				require.True(t, trace.IsConnectionProblem(err))
				require.Nil(t, conn)
			},
		},
		{
			name:      "successfully dial local site",
			cluster:   cluster,
			localSite: testRemoteSite{conn: fakeConn{}},
			assertion: func(t *testing.T, conn net.Conn, err error) {
				require.NoError(t, err)
				require.NotNil(t, conn)
			},
		},

		{
			name:      "default to dialing local site",
			localSite: testRemoteSite{conn: fakeConn{}},
			assertion: func(t *testing.T, conn net.Conn, err error) {
				require.NoError(t, err)
				require.NotNil(t, conn)
			},
		},
		{
			name:    "failure to dial remote site",
			cluster: "leaf",
			tunnel: tunnel{
				site: &testRemoteSite{err: trace.ConnectionProblem(context.DeadlineExceeded, "connection refused")},
			},
			assertion: func(t *testing.T, conn net.Conn, err error) {
				require.Error(t, err)
				require.True(t, trace.IsConnectionProblem(err))
				require.Nil(t, conn)
			},
		},
		{
			name:    "unknown cluster",
			cluster: "fake",
			tunnel: tunnel{
				err: trace.NotFound("unknown cluster"),
			},
			assertion: func(t *testing.T, conn net.Conn, err error) {
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
				require.Nil(t, conn)
			},
		},
		{
			name:    "successfully  dial remote site",
			cluster: "leaf",
			tunnel: tunnel{
				site: &testRemoteSite{conn: fakeConn{}},
			},
			assertion: func(t *testing.T, conn net.Conn, err error) {
				require.NoError(t, err)
				require.NotNil(t, conn)
			},
		},
	}

	ctx := context.Background()

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			router := Router{
				clusterName: cluster,
				localSite:   &tt.localSite,
				siteGetter:  tt.tunnel,
				tracer:      tracing.NoopTracer(cluster),
			}

			conn, err := router.DialSite(ctx, tt.cluster, nil, nil)
			tt.assertion(t, conn, err)
		})
	}
}

func makeGitHubServer(t *testing.T, org string) types.Server {
	t.Helper()
	server, err := types.NewGitHubServer(types.GitHubServerMetadata{
		Integration:  org,
		Organization: org,
	})
	require.NoError(t, err)
	return server
}
