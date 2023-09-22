// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type testSite struct {
	cfg   types.ClusterNetworkingConfig
	nodes []types.Server
}

func (t testSite) GetClusterNetworkingConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterNetworkingConfig, error) {
	return t.cfg, nil
}

func (t testSite) GetNodes(ctx context.Context, fn func(n services.Node) bool) ([]types.Server, error) {
	var out []types.Server
	for _, s := range t.nodes {
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
				require.ErrorIs(t, err, trace.NotFound(teleport.NodeIsAmbiguous))
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
			name:         "failure on invalid addresses",
			site:         testSite{cfg: &unambiguousCfg, nodes: servers},
			host:         "lion",
			errAssertion: require.NoError,
			serverAssertion: func(t *testing.T, srv types.Server) {
				require.Empty(t, srv)
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

type tunnel struct {
	reversetunnel.Tunnel

	site reversetunnel.RemoteSite
	err  error
}

func (t tunnel) GetSite(cluster string) (reversetunnel.RemoteSite, error) {
	return t.site, t.err
}

type testRemoteSite struct {
	reversetunnel.RemoteSite
	conn net.Conn
	err  error
}

func (r testRemoteSite) Dial(reversetunnel.DialParams) (net.Conn, error) {
	return r.conn, r.err
}

func (r testRemoteSite) DialAuthServer(reversetunnel.DialParams) (net.Conn, error) {
	return r.conn, r.err
}

type fakeConn struct {
	net.Conn
}

func TestRouter_DialHost(t *testing.T) {
	t.Parallel()

	logger := utils.NewLoggerForTests().WithField(trace.Component, "test")

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

	cases := []struct {
		name      string
		router    Router
		assertion func(t *testing.T, conn net.Conn, err error)
	}{
		{
			name: "failure resolving node",
			router: Router{
				clusterName:    "test",
				log:            logger,
				tracer:         tracing.NoopTracer("test"),
				serverResolver: serverResolver(nil, trace.NotFound(teleport.NodeIsAmbiguous)),
			},
			assertion: func(t *testing.T, conn net.Conn, err error) {
				require.Error(t, err)
				require.Nil(t, conn)
			},
		},
		{
			name: "failure looking up cluster",
			router: Router{
				clusterName: "leaf",
				siteGetter:  tunnel{err: trace.NotFound("unknown cluster")},
				log:         logger,
				tracer:      tracing.NoopTracer("test"),
			},
			assertion: func(t *testing.T, conn net.Conn, err error) {
				require.Error(t, err)
				require.True(t, trace.IsNotFound(err))
				require.Nil(t, conn)
			},
		},
		{
			name: "dial failure",
			router: Router{
				clusterName:    "test",
				log:            logger,
				localSite:      &testRemoteSite{err: trace.ConnectionProblem(context.DeadlineExceeded, "connection refused")},
				tracer:         tracing.NoopTracer("test"),
				serverResolver: serverResolver(srv, nil),
			},
			assertion: func(t *testing.T, conn net.Conn, err error) {
				require.Error(t, err)
				require.True(t, trace.IsConnectionProblem(err))
				require.Nil(t, conn)
			},
		},
		{
			name: "dial success",
			router: Router{
				clusterName:    "test",
				log:            logger,
				localSite:      &testRemoteSite{conn: fakeConn{}},
				tracer:         tracing.NoopTracer("test"),
				serverResolver: serverResolver(srv, nil),
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
			conn, _, err := tt.router.DialHost(ctx, &utils.NetAddr{}, &utils.NetAddr{}, "host", "0", "test", nil, nil)
			tt.assertion(t, conn, err)
		})
	}

}

func TestRouter_DialSite(t *testing.T) {
	t.Parallel()

	const cluster = "test"
	logger := utils.NewLoggerForTests().WithField(trace.Component, cluster)

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
				site: testRemoteSite{err: trace.ConnectionProblem(context.DeadlineExceeded, "connection refused")},
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
				site: testRemoteSite{conn: fakeConn{}},
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
				log:         logger,
				localSite:   &tt.localSite,
				siteGetter:  tt.tunnel,
				tracer:      tracing.NoopTracer(cluster),
			}

			conn, err := router.DialSite(ctx, tt.cluster, nil, nil)
			tt.assertion(t, conn, err)
		})
	}
}
