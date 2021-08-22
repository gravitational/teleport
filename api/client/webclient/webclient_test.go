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

package webclient

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
)

func TestGetTunnelAddr(t *testing.T) {
	ctx := context.Background()
	t.Run("should use TELEPORT_TUNNEL_PUBLIC_ADDR", func(t *testing.T) {
		os.Setenv(defaults.TunnelPublicAddrEnvar, "tunnel.example.com:4024")
		t.Cleanup(func() { os.Unsetenv(defaults.TunnelPublicAddrEnvar) })
		tunnelAddr, err := GetTunnelAddr(ctx, "", true, nil)
		require.NoError(t, err)
		require.Equal(t, "tunnel.example.com:4024", tunnelAddr)
	})
}

func TestTunnelAddr(t *testing.T) {
	type testCase struct {
		proxyAddr          string
		settings           ProxySettings
		expectedTunnelAddr string
	}

	testTunnelAddr := func(tc testCase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			tunnelAddr, err := tunnelAddr(tc.proxyAddr, tc.settings)
			require.NoError(t, err)
			require.Equal(t, tc.expectedTunnelAddr, tunnelAddr)
		}
	}

	t.Run("should use TunnelPublicAddr", testTunnelAddr(testCase{
		proxyAddr: "proxy.example.com",
		settings: ProxySettings{
			SSH: SSHProxySettings{
				TunnelPublicAddr: "tunnel.example.com:4024",
				PublicAddr:       "public.example.com",
				SSHPublicAddr:    "ssh.example.com",
				TunnelListenAddr: "[::]:5024",
			},
		},
		expectedTunnelAddr: "tunnel.example.com:4024",
	}))
	t.Run("should use SSHPublicAddr and TunnelListenAddr", testTunnelAddr(testCase{
		proxyAddr: "proxy.example.com",
		settings: ProxySettings{
			SSH: SSHProxySettings{
				SSHPublicAddr:    "ssh.example.com",
				PublicAddr:       "public.example.com",
				TunnelListenAddr: "[::]:5024",
			},
		},
		expectedTunnelAddr: "ssh.example.com:5024",
	}))
	t.Run("should use PublicAddr and TunnelListenAddr", testTunnelAddr(testCase{
		proxyAddr: "proxy.example.com",
		settings: ProxySettings{
			SSH: SSHProxySettings{
				PublicAddr:       "public.example.com",
				TunnelListenAddr: "[::]:5024",
			},
		},
		expectedTunnelAddr: "public.example.com:5024",
	}))
	t.Run("should use PublicAddr and SSHProxyTunnelListenPort", testTunnelAddr(testCase{
		proxyAddr: "proxy.example.com",
		settings: ProxySettings{
			SSH: SSHProxySettings{
				PublicAddr: "public.example.com",
			},
		},
		expectedTunnelAddr: "public.example.com:3024",
	}))
	t.Run("should use proxyAddr and SSHProxyTunnelListenPort", testTunnelAddr(testCase{
		proxyAddr:          "proxy.example.com",
		settings:           ProxySettings{SSH: SSHProxySettings{}},
		expectedTunnelAddr: "proxy.example.com:3024",
	}))
	t.Run("should use PublicAddr and WebAddrPort if ALPNSNIListenerEnabled was enabled", testTunnelAddr(testCase{
		proxyAddr: "proxy.example.com:443",
		settings: ProxySettings{
			SSH: SSHProxySettings{
				PublicAddr:       "public.example.com",
				TunnelListenAddr: "[::]:5024",
			},
			ALPNSNIListenerEnabled: true,
		},
		expectedTunnelAddr: "public.example.com:443",
	}))
}

func TestExtract(t *testing.T) {
	testCases := []struct {
		addr     string
		hostPort string
		host     string
		port     string
	}{
		{
			addr:     "example.com",
			hostPort: "example.com",
			host:     "example.com",
			port:     "",
		}, {
			addr:     "example.com:443",
			hostPort: "example.com:443",
			host:     "example.com",
			port:     "443",
		}, {
			addr:     "http://example.com:443",
			hostPort: "example.com:443",
			host:     "example.com",
			port:     "443",
		}, {
			addr:     "https://example.com:443",
			hostPort: "example.com:443",
			host:     "example.com",
			port:     "443",
		}, {
			addr:     "tcp://example.com:443",
			hostPort: "example.com:443",
			host:     "example.com",
			port:     "443",
		}, {
			addr:     "file://host/path",
			hostPort: "",
			host:     "",
			port:     "",
		}, {
			addr:     "[::]:443",
			hostPort: "[::]:443",
			host:     "::",
			port:     "443",
		}, {
			addr:     "https://example.com:443/path?query=query#fragment",
			hostPort: "example.com:443",
			host:     "example.com",
			port:     "443",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.addr, func(t *testing.T) {
			hostPort, err := extractHostPort(tc.addr)
			// Expect err if expected value is empty
			require.True(t, (tc.hostPort == "") == (err != nil))
			require.Equal(t, tc.hostPort, hostPort)

			host, err := extractHost(tc.addr)
			// Expect err if expected value is empty
			require.True(t, (tc.host == "") == (err != nil))
			require.Equal(t, tc.host, host)

			port, err := extractPort(tc.addr)
			// Expect err if expected value is empty
			require.True(t, (tc.port == "") == (err != nil))
			require.Equal(t, tc.port, port)
		})
	}
}
