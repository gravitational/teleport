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
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
	apihelpers "github.com/gravitational/teleport/api/testhelpers"
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
	t.Parallel()

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

func TestPingError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc        string
		writeBody   func(t *testing.T, w http.ResponseWriter)
		errContains string
	}{
		{
			desc: "unsuccessful response",
			writeBody: func(t *testing.T, w http.ResponseWriter) {
				err := json.NewEncoder(w).Encode(PingErrorResponse{Error: PingError{Message: "lorem ipsum"}})
				require.NoError(t, err)
			},
			errContains: "lorem ipsum",
		},
		{
			desc: "mangled response",
			writeBody: func(t *testing.T, w http.ResponseWriter) {
				_, err := w.Write([]byte("mangled lorem ipsum"))
				require.NoError(t, err)
			},
			errContains: "invalid character",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.RequestURI != "/webapi/ping" {
					w.WriteHeader(http.StatusNotFound)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				testCase.writeBody(t, w)
			})
			httpSvr := httptest.NewServer(handler)
			defer httpSvr.Close()
			proxyAddr := httpSvr.Listener.Addr().String()

			_, err := Ping(
				&Config{Context: context.Background(), ProxyAddr: proxyAddr, Insecure: true})
			require.ErrorContains(t, err, testCase.errContains)
		})
	}
}

func TestTunnelAddr(t *testing.T) {
	cases := []struct {
		name               string
		settings           ProxySettings
		expectedTunnelAddr string
		setup              func(t *testing.T)
	}{
		{
			name: "should use TunnelPublicAddr",
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
		},
		{
			name: "should use SSHPublicAddr and TunnelListenAddr",
			settings: ProxySettings{
				SSH: SSHProxySettings{
					SSHPublicAddr:    "ssh.example.com",
					PublicAddr:       "public.example.com",
					TunnelListenAddr: "[::]:5024",
					WebListenAddr:    "proxy.example.com",
				},
			},
			expectedTunnelAddr: "ssh.example.com:5024",
		},
		{
			name: "should use PublicAddr and TunnelListenAddr",
			settings: ProxySettings{
				SSH: SSHProxySettings{
					PublicAddr:       "public.example.com",
					TunnelListenAddr: "[::]:5024",
					WebListenAddr:    "proxy.example.com",
				},
			},
			expectedTunnelAddr: "public.example.com:5024",
		},
		{
			name: "should use PublicAddr and SSHProxyTunnelListenPort",
			settings: ProxySettings{
				SSH: SSHProxySettings{
					PublicAddr:    "public.example.com",
					WebListenAddr: "proxy.example.com",
				},
			},
			expectedTunnelAddr: "public.example.com:3024",
		},
		{
			name: "should use WebListenAddr and SSHProxyTunnelListenPort",
			settings: ProxySettings{
				SSH: SSHProxySettings{
					WebListenAddr: "proxy.example.com",
				},
			},
			expectedTunnelAddr: "proxy.example.com:3024",
		},
		{
			name: "should use PublicAddr with ProxyWebPort if TLSRoutingEnabled was enabled",
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
		},
		{
			name: "should use PublicAddr with custom port if TLSRoutingEnabled was enabled",
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
		},
		{
			name: "should use WebListenAddr with custom ProxyWebPort if TLSRoutingEnabled was enabled",
			settings: ProxySettings{
				SSH: SSHProxySettings{
					TunnelListenAddr: "[::]:5024",
					TunnelPublicAddr: "tpa.example.com:3032",
					WebListenAddr:    "proxy.example.com:443",
				},
				TLSRoutingEnabled: true,
			},
			expectedTunnelAddr: "proxy.example.com:443",
		},
		{
			name: "should use WebListenAddr with default https port if TLSRoutingEnabled was enabled",
			settings: ProxySettings{
				SSH: SSHProxySettings{
					TunnelListenAddr: "[::]:5024",
					TunnelPublicAddr: "tpa.example.com:3032",
					WebListenAddr:    "proxy.example.com",
				},
				TLSRoutingEnabled: true,
			},
			expectedTunnelAddr: "proxy.example.com:443",
		},
		{
			name:               "TELEPORT_TUNNEL_PUBLIC_ADDR overrides tunnel address",
			settings:           ProxySettings{},
			expectedTunnelAddr: "tunnel.example.com:4024",
			setup: func(t *testing.T) {
				t.Setenv(defaults.TunnelPublicAddrEnvar, "tunnel.example.com:4024")
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(t)
			}
			tunnelAddr, err := tt.settings.TunnelAddr()
			require.NoError(t, err)
			require.Equal(t, tt.expectedTunnelAddr, tunnelAddr)
		})
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

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

func TestNewWebClientHTTPProxy(t *testing.T) {
	proxyHandler := &apihelpers.ProxyHandler{}
	proxyServer := httptest.NewServer(proxyHandler)
	t.Cleanup(proxyServer.Close)

	localIP, err := apihelpers.GetLocalIP()
	require.NoError(t, err)
	server := apihelpers.MakeTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}), apihelpers.WithTestServerAddress(localIP))
	_, serverPort, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	serverAddr := net.JoinHostPort(localIP, serverPort)
	tests := []struct {
		name               string
		env                map[string]string
		expectedProxyCount int
	}{
		{
			name: "use http proxy",
			env: map[string]string{
				"HTTPS_PROXY": proxyServer.URL,
			},
			expectedProxyCount: 1,
		},
		{
			name: "ignore proxy when no_proxy is set",
			env: map[string]string{
				"HTTPS_PROXY": proxyServer.URL,
				"NO_PROXY":    "*",
			},
			expectedProxyCount: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(proxyHandler.Reset)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			client, err := newWebClient(&Config{
				Context:   ctx,
				ProxyAddr: "localhost:3080", // addr doesn't matter, it won't be used
				Insecure:  true,
			})
			require.NoError(t, err)

			resp, err := client.Get("https://" + serverAddr)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, tc.expectedProxyCount, proxyHandler.Count())
		})
	}
}

func TestSSHProxyHostPort(t *testing.T) {
	t.Parallel()

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

// TestWebClientClosesIdleConnections verifies that all http connections
// are closed when the http.Client created by newWebClient is no longer
// being used.
func TestWebClientClosesIdleConnections(t *testing.T) {
	expectedResponse := &PingResponse{
		Proxy: ProxySettings{
			TLSRoutingEnabled: true,
		},
		ServerVersion:    "1.2.3",
		MinClientVersion: "0.1.2",
		ClusterName:      "test",
	}

	expectedStates := []string{
		http.StateNew.String(), http.StateActive.String(), http.StateClosed.String(), // the https request will fail and cause us to fallback to http
		http.StateNew.String(), http.StateActive.String(), http.StateIdle.String(), http.StateClosed.String(), // the http request should be processed and closed
	}

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/webapi/find":
			json.NewEncoder(w).Encode(expectedResponse)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))

	stateChange := make(chan string, len(expectedStates))
	srv.Config.ConnState = func(conn net.Conn, state http.ConnState) {
		stateChange <- state.String()
	}

	srv.Start()
	t.Cleanup(srv.Close)

	resp, err := Find(&Config{
		Context:   context.Background(),
		ProxyAddr: strings.TrimPrefix(srv.URL, "http://"),
		Insecure:  true,
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expectedResponse, resp))

	var got []string
	for i := range expectedStates {
		select {
		case state := <-stateChange:
			got = append(got, state)
		case <-time.After(3 * time.Second):
			t.Fatalf("timeout waiting for expected connection state %d", i)
		}
	}

	slices.Sort(expectedStates)
	slices.Sort(got)

	require.Equal(t, expectedStates, got)
}
