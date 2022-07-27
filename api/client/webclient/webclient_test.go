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
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
)

func newPingHandler(path string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.RequestURI != path {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(PingResponse{ServerVersion: "test"})
	})
}

func TestPlainHttpFallback(t *testing.T) {
	testCases := []struct {
		desc            string
		handler         http.Handler
		actionUnderTest func(addr string, insecure bool) error
	}{
		{
			desc:    "Ping",
			handler: newPingHandler("/webapi/ping"),
			actionUnderTest: func(addr string, insecure bool) error {
				_, err := Ping(
					&Config{Context: context.Background(), ProxyAddr: addr, Insecure: insecure})
				return err
			},
		}, {
			desc:    "Find",
			handler: newPingHandler("/webapi/find"),
			actionUnderTest: func(addr string, insecure bool) error {
				_, err := Find(&Config{Context: context.Background(), ProxyAddr: addr, Insecure: insecure})
				return err
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			t.Run("Allowed on insecure & loopback", func(t *testing.T) {
				httpSvr := httptest.NewServer(testCase.handler)
				defer httpSvr.Close()

				err := testCase.actionUnderTest(httpSvr.Listener.Addr().String(), true /* insecure */)
				require.NoError(t, err)
			})

			t.Run("Denied on secure", func(t *testing.T) {
				httpSvr := httptest.NewServer(testCase.handler)
				defer httpSvr.Close()

				err := testCase.actionUnderTest(httpSvr.Listener.Addr().String(), false /* secure */)
				require.Error(t, err)
			})

			t.Run("Denied on non-loopback", func(t *testing.T) {
				nonLoopbackSvr := httptest.NewUnstartedServer(testCase.handler)

				// replace the test-supplied loopback listener with the first available
				// non-loopback address
				nonLoopbackSvr.Listener.Close()
				l, err := net.Listen("tcp", "0.0.0.0:0")
				require.NoError(t, err)
				nonLoopbackSvr.Listener = l
				nonLoopbackSvr.Start()
				defer nonLoopbackSvr.Close()

				err = testCase.actionUnderTest(nonLoopbackSvr.Listener.Addr().String(), true /* insecure */)
				require.Error(t, err)
			})
		})
	}
}

func TestGetTunnelAddr(t *testing.T) {
	t.Setenv(defaults.TunnelPublicAddrEnvar, "tunnel.example.com:4024")
	tunnelAddr, err := GetTunnelAddr(&Config{Context: context.Background(), ProxyAddr: "", Insecure: false})
	require.NoError(t, err)
	require.Equal(t, "tunnel.example.com:4024", tunnelAddr)
}

func TestTunnelAddr(t *testing.T) {
	type testCase struct {
		settings           ProxySettings
		expectedTunnelAddr string
	}

	testTunnelAddr := func(tc testCase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			tunnelAddr, err := tc.settings.tunnelProxyAddr()
			require.NoError(t, err)
			require.Equal(t, tc.expectedTunnelAddr, tunnelAddr)
		}
	}

	t.Run("should use TunnelPublicAddr", testTunnelAddr(testCase{
		settings: ProxySettings{
			SSH: SSHProxySettings{
				TunnelPublicAddr: "tunnel.example.com:4024",
				PublicAddr:       "public.example.com",
				SSHPublicAddr:    "ssh.example.com",
				TunnelListenAddr: "[::]:5024",
				WebListenAddr:    "proxy.example.com",
			},
		},
		expectedTunnelAddr: "tunnel.example.com:4024",
	}))
	t.Run("should use SSHPublicAddr and TunnelListenAddr", testTunnelAddr(testCase{
		settings: ProxySettings{
			SSH: SSHProxySettings{
				SSHPublicAddr:    "ssh.example.com",
				PublicAddr:       "public.example.com",
				TunnelListenAddr: "[::]:5024",
				WebListenAddr:    "proxy.example.com",
			},
		},
		expectedTunnelAddr: "ssh.example.com:5024",
	}))
	t.Run("should use PublicAddr and TunnelListenAddr", testTunnelAddr(testCase{
		settings: ProxySettings{
			SSH: SSHProxySettings{
				PublicAddr:       "public.example.com",
				TunnelListenAddr: "[::]:5024",
				WebListenAddr:    "proxy.example.com",
			},
		},
		expectedTunnelAddr: "public.example.com:5024",
	}))
	t.Run("should use PublicAddr and SSHProxyTunnelListenPort", testTunnelAddr(testCase{
		settings: ProxySettings{
			SSH: SSHProxySettings{
				PublicAddr:    "public.example.com",
				WebListenAddr: "proxy.example.com",
			},
		},
		expectedTunnelAddr: "public.example.com:3024",
	}))
	t.Run("should use WebListenAddr and SSHProxyTunnelListenPort", testTunnelAddr(testCase{
		settings: ProxySettings{
			SSH: SSHProxySettings{
				WebListenAddr: "proxy.example.com",
			},
		},
		expectedTunnelAddr: "proxy.example.com:3024",
	}))
	t.Run("should use PublicAddr with ProxyWebPort if TLSRoutingEnabled was enabled", testTunnelAddr(testCase{
		settings: ProxySettings{
			SSH: SSHProxySettings{
				PublicAddr:       "public.example.com",
				TunnelListenAddr: "[::]:5024",
				TunnelPublicAddr: "tpa.example.com:3032",
				WebListenAddr:    "proxy.example.com:443",
			},
			TLSRoutingEnabled: true,
		},
		expectedTunnelAddr: "public.example.com:443",
	}))
	t.Run("should use PublicAddr with custom port if TLSRoutingEnabled was enabled", testTunnelAddr(testCase{
		settings: ProxySettings{
			SSH: SSHProxySettings{
				PublicAddr:       "public.example.com:443",
				TunnelListenAddr: "[::]:5024",
				TunnelPublicAddr: "tpa.example.com:3032",
				WebListenAddr:    "proxy.example.com:443",
			},
			TLSRoutingEnabled: true,
		},
		expectedTunnelAddr: "public.example.com:443",
	}))
	t.Run("should use WebListenAddr with custom ProxyWebPort if TLSRoutingEnabled was enabled", testTunnelAddr(testCase{
		settings: ProxySettings{
			SSH: SSHProxySettings{
				TunnelListenAddr: "[::]:5024",
				TunnelPublicAddr: "tpa.example.com:3032",
				WebListenAddr:    "proxy.example.com:443",
			},
			TLSRoutingEnabled: true,
		},
		expectedTunnelAddr: "proxy.example.com:443",
	}))
	t.Run("should use WebListenAddr with default https port if TLSRoutingEnabled was enabled", testTunnelAddr(testCase{
		settings: ProxySettings{
			SSH: SSHProxySettings{
				TunnelListenAddr: "[::]:5024",
				TunnelPublicAddr: "tpa.example.com:3032",
				WebListenAddr:    "proxy.example.com",
			},
			TLSRoutingEnabled: true,
		},
		expectedTunnelAddr: "proxy.example.com:443",
	}))
}

func TestParse(t *testing.T) {
	testCases := []struct {
		addr     string
		hostPort string
		host     string
		port     int
	}{
		{
			addr:     "example.com",
			hostPort: "example.com",
			host:     "example.com",
			port:     0,
		}, {
			addr:     "example.com:443",
			hostPort: "example.com:443",
			host:     "example.com",
			port:     443,
		}, {
			addr:     "http://example.com:443",
			hostPort: "example.com:443",
			host:     "example.com",
			port:     443,
		}, {
			addr:     "https://example.com:443",
			hostPort: "example.com:443",
			host:     "example.com",
			port:     443,
		}, {
			addr:     "tcp://example.com:443",
			hostPort: "example.com:443",
			host:     "example.com",
			port:     443,
		}, {
			addr:     "file://host/path",
			hostPort: "",
			host:     "",
			port:     0,
		}, {
			addr:     "[::]:443",
			hostPort: "[::]:443",
			host:     "::",
			port:     443,
		}, {
			addr:     "https://example.com:443/path?query=query#fragment",
			hostPort: "example.com:443",
			host:     "example.com",
			port:     443,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.addr, func(t *testing.T) {
			hostPort, err := parseAndJoinHostPort(tc.addr)
			if tc.hostPort == "" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.hostPort, hostPort)
			}

			host, _, err := ParseHostPort(tc.addr)
			if tc.host == "" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.host, host)
			}

			port, err := parsePort(tc.addr)
			if tc.port == 0 {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.port, port)
			}
		})
	}
}

func TestNewWebClientRespectHTTPProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "fakeproxy.example.com:9999")
	client, err := newWebClient(&Config{
		Context:   context.Background(),
		ProxyAddr: "localhost:3080",
	})
	require.NoError(t, err)
	// resp should be nil, so there will be no body to close.
	//nolint:bodyclose
	resp, err := client.Get("https://fakedomain.example.com")
	// Client should try to proxy through nonexistent server at localhost.
	require.Error(t, err, "GET unexpectedly succeeded: %+v", resp)
	require.Contains(t, err.Error(), "proxyconnect")
	require.Contains(t, err.Error(), "lookup fakeproxy.example.com")
	require.Contains(t, err.Error(), "no such host")
}

func TestNewWebClientNoProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "fakeproxy.example.com:9999")
	t.Setenv("NO_PROXY", "fakedomain.example.com")
	client, err := newWebClient(&Config{
		Context:   context.Background(),
		ProxyAddr: "localhost:3080",
	})
	require.NoError(t, err)
	//nolint:bodyclose
	resp, err := client.Get("https://fakedomain.example.com")
	require.Error(t, err, "GET unexpectedly succeeded: %+v", resp)
	require.NotContains(t, err.Error(), "proxyconnect")
	require.Contains(t, err.Error(), "lookup fakedomain.example.com")
	require.Contains(t, err.Error(), "no such host")
}

func TestSSHProxyHostPort(t *testing.T) {
	tests := []struct {
		testName        string
		inProxySettings ProxySettings
		outHost         string
		outPort         string
	}{
		{
			testName: "TLS routing enabled, web public addr",
			inProxySettings: ProxySettings{
				SSH: SSHProxySettings{
					PublicAddr:    "proxy.example.com:443",
					WebListenAddr: "127.0.0.1:3080",
				},
				TLSRoutingEnabled: true,
			},
			outHost: "proxy.example.com",
			outPort: "443",
		},
		{
			testName: "TLS routing enabled, web public addr with listen addr",
			inProxySettings: ProxySettings{
				SSH: SSHProxySettings{
					PublicAddr:    "proxy.example.com",
					WebListenAddr: "127.0.0.1:443",
				},
				TLSRoutingEnabled: true,
			},
			outHost: "proxy.example.com",
			outPort: "443",
		},
		{
			testName: "TLS routing enabled, web listen addr",
			inProxySettings: ProxySettings{
				SSH: SSHProxySettings{
					WebListenAddr: "127.0.0.1:3080",
				},
				TLSRoutingEnabled: true,
			},
			outHost: "127.0.0.1",
			outPort: "3080",
		},
		{
			testName: "TLS routing disabled, SSH public addr",
			inProxySettings: ProxySettings{
				SSH: SSHProxySettings{
					SSHPublicAddr: "ssh.example.com:3023",
					PublicAddr:    "proxy.example.com:443",
					ListenAddr:    "127.0.0.1:3023",
				},
				TLSRoutingEnabled: false,
			},
			outHost: "ssh.example.com",
			outPort: "3023",
		},
		{
			testName: "TLS routing disabled, web public addr",
			inProxySettings: ProxySettings{
				SSH: SSHProxySettings{
					PublicAddr: "proxy.example.com:443",
					ListenAddr: "127.0.0.1:3023",
				},
				TLSRoutingEnabled: false,
			},
			outHost: "proxy.example.com",
			outPort: "3023",
		},
		{
			testName: "TLS routing disabled, SSH listen addr",
			inProxySettings: ProxySettings{
				SSH: SSHProxySettings{
					ListenAddr: "127.0.0.1:3023",
				},
				TLSRoutingEnabled: false,
			},
			outHost: "127.0.0.1",
			outPort: "3023",
		},
	}
	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			host, port, err := test.inProxySettings.SSHProxyHostPort()
			require.NoError(t, err)
			require.Equal(t, test.outHost, host)
			require.Equal(t, test.outPort, port)
		})
	}
}
