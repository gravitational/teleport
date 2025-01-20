// Copyright 2023 Gravitational, Inc
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

package utils

import (
	"cmp"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// NOTE: much of the details of the behavior of this type is tested in lib/proxy as part
// of the main router test coverage.

// TestSSHRouteMatcherHostnameMatching verifies the expected behavior of the custom ssh
// hostname matching logic.
func TestSSHRouteMatcherHostnameMatching(t *testing.T) {
	tts := []struct {
		desc        string
		principal   string
		target      string
		insensitive bool
		match       bool
	}{
		{
			desc:        "upper-eq",
			principal:   "Foo",
			target:      "Foo",
			insensitive: true,
			match:       true,
		},
		{
			desc:        "lower-eq",
			principal:   "foo",
			target:      "foo",
			insensitive: true,
			match:       true,
		},
		{
			desc:        "lower-target-match",
			principal:   "Foo",
			target:      "foo",
			insensitive: true,
			match:       true,
		},
		{
			desc:        "upper-target-mismatch",
			principal:   "foo",
			target:      "Foo",
			insensitive: true,
			match:       false,
		},
		{
			desc:        "upper-mismatch",
			principal:   "Foo",
			target:      "fOO",
			insensitive: true,
			match:       false,
		},
		{
			desc:        "non-ascii-match",
			principal:   "🌲",
			target:      "🌲",
			insensitive: true,
			match:       true,
		},
		{
			desc:        "non-ascii-mismatch",
			principal:   "🌲",
			target:      "🔥",
			insensitive: true,
			match:       false,
		},
		{
			desc:        "sensitive-match",
			principal:   "Foo",
			target:      "Foo",
			insensitive: false,
			match:       true,
		},
		{
			desc:        "sensitive-mismatch",
			principal:   "Foo",
			target:      "foo",
			insensitive: false,
			match:       false,
		},
	}

	for _, tt := range tts {
		matcher := NewSSHRouteMatcher(tt.target, "", tt.insensitive)
		require.Equal(t, tt.match, matcher.routeToHostname(tt.principal), "desc=%q", tt.desc)
	}
}

type mockRouteableServer struct {
	name       string
	hostname   string
	addr       string
	useTunnel  bool
	publicAddr []string
}

func (m mockRouteableServer) GetName() string {
	return m.name
}

func (m mockRouteableServer) GetHostname() string {
	return m.hostname
}

func (m mockRouteableServer) GetAddr() string {
	return m.addr
}

func (m mockRouteableServer) GetUseTunnel() bool {
	return m.useTunnel
}

func (m mockRouteableServer) GetPublicAddrs() []string {
	return m.publicAddr
}

func TestRouteToServer(t *testing.T) {
	t.Parallel()
	testUUID := uuid.NewString()

	matchAddrServer := mockRouteableServer{
		name:       "test",
		addr:       "example.com:1111",
		publicAddr: []string{"node:1234", "public.example.com:1111"},
	}

	tests := []struct {
		name    string
		matcher SSHRouteMatcher
		server  RouteableServer
		assert  require.BoolAssertionFunc
	}{
		{
			name:    "no match",
			matcher: NewSSHRouteMatcher(testUUID, "", true),
			server: mockRouteableServer{
				name:       "test",
				addr:       "localhost",
				hostname:   "example.com",
				publicAddr: []string{"example.com"},
			},
			assert: require.False,
		},
		{
			name:    "match by server name",
			matcher: NewSSHRouteMatcher(testUUID, "", true),
			server: mockRouteableServer{
				name:       testUUID,
				addr:       "localhost",
				hostname:   "example.com",
				publicAddr: []string{"example.com"},
			},
			assert: require.True,
		},
		{
			name:    "match by hostname over tunnel",
			matcher: NewSSHRouteMatcher("example.com", "", true),
			server: mockRouteableServer{
				name:       testUUID,
				addr:       "addr.example.com",
				hostname:   "example.com",
				publicAddr: []string{"public.example.com"},
				useTunnel:  true,
			},
			assert: require.True,
		},
		{
			name:    "mismatch hostname over tunnel",
			matcher: NewSSHRouteMatcher("example.com", "", true),
			server: mockRouteableServer{
				name:       testUUID,
				addr:       "example.com",
				hostname:   "fake.example.com",
				publicAddr: []string{"example.com"},
				useTunnel:  true,
			},
			assert: require.False,
		},
		{
			name:    "match addr",
			matcher: NewSSHRouteMatcher("example.com", "1111", true),
			server:  matchAddrServer,
			assert:  require.True,
		},
		{
			name:    "match addr with empty port",
			matcher: NewSSHRouteMatcher("example.com", "", true),
			server:  matchAddrServer,
			assert:  require.True,
		},
		{
			name:    "mismatch addr with wrong port",
			matcher: NewSSHRouteMatcher("example.com", "2222", true),
			server:  matchAddrServer,
			assert:  require.False,
		},
		{
			name:    "match first public addr",
			matcher: NewSSHRouteMatcher("node", "1234", true),
			server:  matchAddrServer,
			assert:  require.True,
		},
		{
			name:    "match second public addr",
			matcher: NewSSHRouteMatcher("public.example.com", "1111", true),
			server:  matchAddrServer,
			assert:  require.True,
		},
		{
			name:    "match public addr with empty port",
			matcher: NewSSHRouteMatcher("public.example.com", "", true),
			server:  matchAddrServer,
			assert:  require.True,
		},
		{
			name:    "mismatch public addr with wrong port",
			matcher: NewSSHRouteMatcher("public.example.com", "2222", true),
			server:  matchAddrServer,
			assert:  require.False,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.assert(t, tc.matcher.RouteToServer(tc.server))
		})
	}
}

type mockHostResolver struct {
	ips []string
}

func (r mockHostResolver) LookupHost(ctx context.Context, host string) (addrs []string, err error) {
	return r.ips, nil
}

// TestSSHRouteMatcherScoring verifies the expected scoring behavior of SSHRouteMatcher.
func TestSSHRouteMatcherScoring(t *testing.T) {
	t.Parallel()

	// set up matcher with mock resolver in order to control ips
	matcher, err := NewSSHRouteMatcherFromConfig(SSHRouteMatcherConfig{
		Host: "foo.example.com",
		Resolver: mockHostResolver{
			ips: []string{
				"1.2.3.4",
				"4.5.6.7",
			},
		},
	})
	require.NoError(t, err)

	tts := []struct {
		desc     string
		hostname string
		name     string
		addrs    []string
		score    int
	}{
		{
			desc:     "multi factor match",
			hostname: "foo.example.com",
			addrs: []string{
				"1.2.3.4:0",
			},
			score: directMatch,
		},
		{
			desc:     "ip match only",
			hostname: "bar.example.com",
			addrs: []string{
				"1.2.3.4:0",
			},
			score: indirectMatch,
		},
		{
			desc:     "hostname match only",
			hostname: "foo.example.com",
			addrs: []string{
				"7.7.7.7:0",
			},
			score: directMatch,
		},
		{
			desc:     "not match",
			hostname: "bar.example.com",
			addrs: []string{
				"0.0.0.0:0",
				"1.1.1.1:0",
			},
			score: notMatch,
		},
		{
			desc:     "non-uuid name",
			name:     "foo.example.com",
			hostname: "foo.com",
			addrs: []string{
				"0.0.0.0:0",
				"1.1.1.1:0",
			},
			score: indirectMatch,
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			name := cmp.Or(tt.name, uuid.NewString())
			score := matcher.RouteToServerScore(mockRouteableServer{
				name:       name,
				hostname:   tt.hostname,
				publicAddr: tt.addrs,
			})

			require.Equal(t, tt.score, score)
		})
	}
}

type recordingHostResolver struct {
	didLookup bool
}

func (r *recordingHostResolver) LookupHost(ctx context.Context, host string) (addrs []string, err error) {
	r.didLookup = true
	return nil, nil
}

// TestDisableUnqualifiedLookups verifies that unqualified lookups being disabled results
// in single-element/tld style hostname targets not being resolved.
func TestDisableUnqualifiedLookups(t *testing.T) {
	tts := []struct {
		desc   string
		target string
		lookup bool
	}{
		{
			desc:   "qualified hostname",
			target: "example.com",
			lookup: true,
		},
		{
			desc:   "unqualified hostname",
			target: "example",
			lookup: false,
		},
		{
			desc:   "localhost",
			target: "localhost",
			lookup: false,
		},
		{
			desc:   "foo.localhost",
			target: "foo.localhost",
			lookup: true,
		},
		{
			desc:   "uuid",
			target: uuid.NewString(),
			lookup: false,
		},
		{
			desc:   "qualified uuid",
			target: "foo." + uuid.NewString(),
			lookup: true,
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			resolver := &recordingHostResolver{}
			_, err := NewSSHRouteMatcherFromConfig(SSHRouteMatcherConfig{
				Host:                      tt.target,
				Resolver:                  resolver,
				DisableUnqualifiedLookups: true,
			})
			require.NoError(t, err)
			require.Equal(t, tt.lookup, resolver.didLookup)
		})
	}
}
