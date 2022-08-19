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
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/constants"
	"github.com/stretchr/testify/require"
)

func TestALPNConnUpgradeTest(t *testing.T) {
	tests := []struct {
		name           string
		protos         []string
		expectedResult bool
	}{
		{
			name:           "upgrade required",
			protos:         nil, // Use nil for NextProtos to simulate no ALPN support.
			expectedResult: true,
		},
		{
			name:           "upgrade not required (proto neogotiated)",
			protos:         []string{constants.ALPNSNIProtocolReverseTunnel},
			expectedResult: false,
		},
		{
			name:           "upgrade not required (handshake error)",
			protos:         []string{"unknown"},
			expectedResult: false,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			server := mustStartMockALPNServer(t, test.protos)
			require.Equal(t, test.expectedResult, alpnConnUpgradeTest(server.Addr().String(), true, 5*time.Second))
		})
	}
}

func TestALPNConUpgradeDialer(t *testing.T) {
	t.Run("connection upgraded", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTLSServer(mockConnUpgradeHandler(t, constants.ConnectionUpgradeTypeALPN, []byte("hello")))
		addr, err := url.Parse(server.URL)
		require.NoError(t, err)

		dialer := newALPNConnUpgradeDialer(5*time.Second, 5*time.Second, true)
		conn, err := dialer.DialContext(context.TODO(), "tcp", addr.Host)
		require.NoError(t, err)

		data := make([]byte, 100)
		n, err := conn.Read(data)
		require.NoError(t, err)
		require.Equal(t, string(data[:n]), "hello")
	})

	t.Run("connection upgrade API not found", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTLSServer(http.NotFoundHandler())
		addr, err := url.Parse(server.URL)
		require.NoError(t, err)

		dialer := newALPNConnUpgradeDialer(5*time.Second, 5*time.Second, true)
		_, err = dialer.DialContext(context.TODO(), "tcp", addr.Host)
		require.Error(t, err)
	})
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

	cert, err := tls.X509KeyPair(tlsCert, keyPEM)
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, constants.ConnectionUpgradeWebAPI, r.URL.Path)
		require.Equal(t, upgradeType, r.Header.Get(constants.ConnectionUpgradeHeader))

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
		_, err = conn.Write(write)
		require.NoError(t, err)
	})
}
