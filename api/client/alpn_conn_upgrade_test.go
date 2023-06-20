/*
Copyright 2022 Gravitational, Inc.

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

package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/fixtures"
	"github.com/gravitational/teleport/api/testhelpers"
	"github.com/gravitational/teleport/api/utils/pingconn"
)

func TestIsALPNConnUpgradeRequired(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		serverProtos   []string
		insecure       bool
		expectedResult bool
	}{
		{
			name:           "upgrade required (handshake success)",
			serverProtos:   nil, // Use nil for NextProtos to simulate no ALPN support.
			insecure:       true,
			expectedResult: true,
		},
		{
			name:           "upgrade not required (proto negotiated)",
			serverProtos:   []string{string(constants.ALPNSNIProtocolReverseTunnel)},
			insecure:       true,
			expectedResult: false,
		},
		{
			name:           "upgrade required (handshake with no ALPN error)",
			serverProtos:   []string{"unknown"},
			insecure:       true,
			expectedResult: true,
		},
		{
			name:           "upgrade not required (other handshake error)",
			serverProtos:   []string{string(constants.ALPNSNIProtocolReverseTunnel)},
			insecure:       false, // to cause handshake error
			expectedResult: false,
		},
	}

	ctx := context.Background()
	forwardProxy, forwardProxyURL := mustStartForwardProxy(t)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := mustStartMockALPNServer(t, test.serverProtos)
			t.Run("direct", func(t *testing.T) {
				require.Equal(t, test.expectedResult, IsALPNConnUpgradeRequired(ctx, server.Addr().String(), test.insecure))
			})

			t.Run("with ProxyURL", func(t *testing.T) {
				countBeforeTest := forwardProxy.Count()
				require.Equal(t, test.expectedResult, IsALPNConnUpgradeRequired(ctx, server.Addr().String(), test.insecure, withProxyURL(forwardProxyURL)))
				require.Equal(t, countBeforeTest+1, forwardProxy.Count())
			})
		})
	}
}

func TestIsALPNConnUpgradeRequiredByEnv(t *testing.T) {
	t.Parallel()

	addr := "example.teleport.com:443"
	tests := []struct {
		name     string
		envValue string
		require  require.BoolAssertionFunc
	}{
		{
			name:     "upgraded required (for all addr)",
			envValue: "yes",
			require:  require.True,
		},
		{
			name:     "upgraded required (for target addr)",
			envValue: "0;example.teleport.com:443=1",
			require:  require.True,
		},
		{
			name:     "upgraded not required (for all addr)",
			envValue: "false",
			require:  require.False,
		},
		{
			name:     "upgraded not required (no addr match)",
			envValue: "another.teleport.com:443=true",
			require:  require.False,
		},
		{
			name:     "upgraded not required (for target addr)",
			envValue: "another.teleport.com:443=true,example.teleport.com:443=false",
			require:  require.False,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.require(t, isALPNConnUpgradeRequiredByEnv(addr, test.envValue))
		})
	}
}

func TestALPNConnUpgradeDialer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		serverHandler http.Handler
		withPing      bool
		wantError     bool
	}{
		{
			name:          "connection upgrade",
			serverHandler: mockConnUpgradeHandler(t, constants.WebAPIConnUpgradeTypeALPN, []byte("hello")),
		},
		{
			name:          "connection upgrade with ping",
			serverHandler: mockConnUpgradeHandler(t, constants.WebAPIConnUpgradeTypeALPNPing, []byte("hello")),
			withPing:      true,
		},
		{
			name:          "connection upgrade API not found",
			serverHandler: http.NotFoundHandler(),
			wantError:     true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			server := httptest.NewTLSServer(test.serverHandler)
			t.Cleanup(server.Close)
			addr, err := url.Parse(server.URL)
			require.NoError(t, err)
			pool := x509.NewCertPool()
			pool.AddCert(server.Certificate())

			tlsConfig := &tls.Config{RootCAs: pool}
			directDialer := newDirectDialer(0, 5*time.Second)

			t.Run("direct", func(t *testing.T) {
				dialer := newALPNConnUpgradeDialer(directDialer, tlsConfig, test.withPing)
				conn, err := dialer.DialContext(ctx, "tcp", addr.Host)
				if test.wantError {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
				defer conn.Close()

				mustReadConnData(t, conn, "hello")
			})

			t.Run("with ProxyURL", func(t *testing.T) {
				forwardProxy, forwardProxyURL := mustStartForwardProxy(t)
				countBeforeTest := forwardProxy.Count()

				proxyURLDialer := newProxyURLDialer(forwardProxyURL, directDialer)
				dialer := newALPNConnUpgradeDialer(proxyURLDialer, tlsConfig, test.withPing)
				conn, err := dialer.DialContext(ctx, "tcp", addr.Host)
				if test.wantError {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
				defer conn.Close()

				mustReadConnData(t, conn, "hello")
				require.Equal(t, countBeforeTest+1, forwardProxy.Count())
			})
		})
	}
}

func mustReadConnData(t *testing.T, conn net.Conn, wantText string) {
	data := make([]byte, len(wantText)*2)
	n, err := conn.Read(data)
	require.NoError(t, err)
	require.Equal(t, len(wantText), n)
	require.Equal(t, string(data[:n]), wantText)
}

type mockALPNServer struct {
	net.Listener
	cert            tls.Certificate
	supportedProtos []string
}

func (m *mockALPNServer) serve(ctx context.Context, t *testing.T) {
	config := &tls.Config{
		NextProtos:   m.supportedProtos,
		Certificates: []tls.Certificate{m.cert},
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := m.Accept()
		if errors.Is(err, net.ErrClosed) {
			return
		}

		go func() {
			clientConn := tls.Server(conn, config)
			clientConn.HandshakeContext(ctx)
			clientConn.Close()
		}()
	}
}

func mustStartMockALPNServer(t *testing.T, supportedProtos []string) *mockALPNServer {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		listener.Close()
	})

	cert, err := tls.X509KeyPair([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	require.NoError(t, err)

	m := &mockALPNServer{
		Listener:        listener,
		cert:            cert,
		supportedProtos: supportedProtos,
	}
	go m.serve(ctx, t)
	return m
}

// mockConnUpgradeHandler mocks the server side implementation to handle an
// upgrade request and sends back some data inside the tunnel.
func mockConnUpgradeHandler(t *testing.T, upgradeType string, write []byte) http.Handler {
	t.Helper()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, constants.WebAPIConnUpgrade, r.URL.Path)
		require.Equal(t, upgradeType, r.Header.Get(constants.WebAPIConnUpgradeHeader))
		require.Equal(t, constants.WebAPIConnUpgradeConnectionType, r.Header.Get(constants.WebAPIConnUpgradeConnectionHeader))

		hj, ok := w.(http.Hijacker)
		require.True(t, ok)

		conn, _, err := hj.Hijack()
		require.NoError(t, err)
		defer conn.Close()

		// Upgrade response.
		response := &http.Response{
			StatusCode: http.StatusSwitchingProtocols,
			ProtoMajor: 1,
			ProtoMinor: 1,
		}
		require.NoError(t, response.Write(conn))

		// Upgraded.
		switch upgradeType {
		case constants.WebAPIConnUpgradeTypeALPNPing:
			// Wrap conn with Ping and write some pings.
			pingConn := pingconn.New(conn)
			pingConn.WritePing()
			_, err = pingConn.Write(write)
			require.NoError(t, err)
			pingConn.WritePing()

		default:
			_, err = conn.Write(write)
			require.NoError(t, err)
		}
	})
}

func mustStartForwardProxy(t *testing.T) (*testhelpers.ProxyHandler, *url.URL) {
	t.Helper()

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		listener.Close()
	})

	url, err := url.Parse("http://" + listener.Addr().String())
	require.NoError(t, err)

	handler := &testhelpers.ProxyHandler{}
	go http.Serve(listener, handler)
	return handler, url
}
