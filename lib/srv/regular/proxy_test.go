/*
Copyright 2016 Gravitational, Inc.

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

package regular

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
)

func TestParseProxyRequest(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc, req string
		expected  proxySubsysRequest
	}{
		{
			desc: "proxy request for a host:port",
			req:  "proxy:host:22",
			expected: proxySubsysRequest{
				namespace:   "",
				host:        "host",
				port:        "22",
				clusterName: "",
			},
		},
		{
			desc: "similar request, just with '@' at the end (missing site)",
			req:  "proxy:host:22@",
			expected: proxySubsysRequest{
				namespace:   "",
				host:        "host",
				port:        "22",
				clusterName: "",
			},
		},
		{
			desc: "proxy request for just the sitename",
			req:  "proxy:@moon",
			expected: proxySubsysRequest{
				namespace:   "",
				host:        "",
				port:        "",
				clusterName: "moon",
			},
		},
		{
			desc: "proxy request for the host:port@sitename",
			req:  "proxy:station:100@moon",
			expected: proxySubsysRequest{
				namespace:   "",
				host:        "station",
				port:        "100",
				clusterName: "moon",
			},
		},
		{
			desc: "proxy request for the host:port@namespace@cluster",
			req:  "proxy:station:100@system@moon",
			expected: proxySubsysRequest{
				namespace:   "system",
				host:        "station",
				port:        "100",
				clusterName: "moon",
			},
		},
	}

	for i, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			if tt.expected.namespace == "" {
				// test cases without a defined namespace are testing for
				// the presence of the default namespace; namespace should
				// never actually be empty.
				tt.expected.namespace = apidefaults.Namespace
			}
			req, err := parseProxySubsysRequest(tt.req)
			require.NoError(t, err, "Test case %d: req=%s, expected=%+v", i, tt.req, tt.expected)
			require.Equal(t, tt.expected, req, "Test case %d: req=%s, expected=%+v", i, tt.req, tt.expected)
		})
	}
}

func TestParseBadRequests(t *testing.T) {
	t.Parallel()

	server := &Server{
		hostname:  "redhorse",
		proxyMode: true,
	}

	ctx := &srv.ServerContext{}

	testCases := []struct {
		desc  string
		input string
	}{
		{desc: "empty request", input: "proxy:"},
		{desc: "missing hostname", input: "proxy::80"},
		{desc: "missing hostname and missing cluster name", input: "proxy:@"},
		{desc: "just random string", input: "this is bad string"},
	}
	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			subsystem, err := parseProxySubsys(tt.input, server, ctx)
			require.Error(t, err, "test case: %q", tt.input)
			require.Nil(t, subsystem, "test case: %q", tt.input)
		})
	}
}

type nodeGetter struct {
	servers []types.Server
}

func (n nodeGetter) GetNodes(fn func(n services.Node) bool) []types.Server {
	var servers []types.Server
	for _, s := range n.servers {
		if fn(s) {
			servers = append(servers, s)
		}
	}

	return servers
}

func TestProxySubsys_getMatchingServer(t *testing.T) {
	t.Parallel()

	serverUUID := uuid.NewString()

	setExpiry := func(time time.Time) func(server types.Server) {
		return func(server types.Server) {
			server.SetExpiry(time)
		}
	}

	createServer := func(name string, spec types.ServerSpecV2, opts ...func(server types.Server)) types.Server {
		t.Helper()

		server, err := types.NewServer(name, types.KindNode, spec)
		require.NoError(t, err)

		for _, opt := range opts {
			opt(server)
		}

		return server
	}

	servers := []types.Server{
		createServer(serverUUID, types.ServerSpecV2{
			Hostname: "127.0.0.1",
			Addr:     "127.0.0.1:80",
		}, setExpiry(time.Now().Add(-time.Hour))),
		createServer("server2", types.ServerSpecV2{
			Hostname: "localhost",
			Addr:     "127.0.0.1:80",
		}, setExpiry(time.Now().Add(time.Hour*24))),
		createServer("server3", types.ServerSpecV2{
			Hostname: serverUUID,
			Addr:     "127.0.0.1:",
		}),
	}

	cases := []struct {
		desc         string
		req          proxySubsysRequest
		strategy     types.RoutingStrategy
		servers      []types.Server
		expectError  require.ErrorAssertionFunc
		expectServer func(servers []types.Server) types.Server
	}{
		{
			desc:        "No matches found",
			expectError: require.NoError,
		},
		{
			desc:        "No matches found for UUID host",
			expectError: require.Error,
			servers: []types.Server{createServer(uuid.NewString(), types.ServerSpecV2{
				Addr: "127.0.0.1:0",
			})},
			req: proxySubsysRequest{
				host: uuid.NewString(),
			},
		},
		{
			desc:        "Match by UUID",
			expectError: require.NoError,
			expectServer: func(servers []types.Server) types.Server {
				return servers[0]
			},
			servers: servers,
			req: proxySubsysRequest{
				host: serverUUID,
			},
		},
		{
			desc:        "Match Tunnel By Host Only",
			expectError: require.NoError,
			expectServer: func(servers []types.Server) types.Server {
				return servers[0]
			},
			servers: []types.Server{
				createServer("server1", types.ServerSpecV2{
					Addr:      "127.0.0.1",
					Hostname:  "127.0.0.1",
					UseTunnel: true,
				}),
				createServer("server2", types.ServerSpecV2{
					Hostname:  "localhost",
					Addr:      "127.0.0.1:80",
					UseTunnel: true,
				}),
			},
			req: proxySubsysRequest{
				host: "127.0.0.1",
				port: "80",
			},
		},
		{
			desc:        "Match by IP",
			expectError: require.NoError,
			expectServer: func(servers []types.Server) types.Server {
				return servers[1]
			},
			servers: []types.Server{
				createServer("server1", types.ServerSpecV2{
					Addr:     "127.0.0.1:0",
					Hostname: "127.0.0.1",
				}),
				createServer("server2", types.ServerSpecV2{
					Hostname: "localhost",
					Addr:     "127.0.0.1:80",
				}),
			},
			req: proxySubsysRequest{
				host: "127.0.0.1",
				port: "80",
			},
		},
		{
			desc:        "Match by hostname",
			expectError: require.NoError,
			expectServer: func(servers []types.Server) types.Server {
				return servers[1]
			},
			servers: []types.Server{
				createServer("server1", types.ServerSpecV2{
					Addr:     "127.0.0.1:0",
					Hostname: "localhost",
				}),
				createServer("server2", types.ServerSpecV2{
					Hostname: "localhost",
					Addr:     "127.0.0.1:80",
				}),
			},
			req: proxySubsysRequest{
				host: "localhost",
				port: "80",
			},
		},
		{
			desc:        "Ambiguous match",
			expectError: require.Error,
			servers:     servers,
			req: proxySubsysRequest{
				host: "localhost",
			},
		},
		{
			desc:        "Most Recent match",
			expectError: require.NoError,
			expectServer: func(servers []types.Server) types.Server {
				return servers[1]
			},
			servers:  servers,
			strategy: types.RoutingStrategy_MOST_RECENT,
			req: proxySubsysRequest{
				host: "localhost",
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.desc, func(t *testing.T) {
			subsystem := proxySubsys{
				proxySubsysRequest: tt.req,
				srv:                &Server{},
			}

			server, err := subsystem.getMatchingServer(nodeGetter{tt.servers}, tt.strategy)
			tt.expectError(t, err)
			if tt.expectServer != nil {
				require.Equal(t, tt.expectServer(tt.servers), server)
			}
		})
	}
}
