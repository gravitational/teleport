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
	server1 := newMockALPNServer(t, nil) // Use nil for NextProtos to simulate no ALPN support.
	server2 := newMockALPNServer(t, []string{constants.ALPNSNIProtocolReverseTunnel})
	server3 := newMockALPNServer(t, []string{"unknown"})

	tests := []struct {
		name           string
		serverAddr     string
		expectedResult bool
	}{
		{
			name:           "upgrade required",
			serverAddr:     server1.listener.Addr().String(),
			expectedResult: true,
		},
		{
			name:           "upgrade not required (proto neogotiated)",
			serverAddr:     server2.listener.Addr().String(),
			expectedResult: false,
		},
		{
			name:           "upgrade not required (handshake error)",
			serverAddr:     server3.listener.Addr().String(),
			expectedResult: false,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, test.expectedResult, alpnConnUpgradeTest(test.serverAddr, true, 5*time.Second))
		})
	}
}

func TestALPNConUpgradeDialer(t *testing.T) {
	t.Run("connection upgraded", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewTLSServer(&mockConnUpgradeHandler{
			Assertions: require.New(t),
			WriteData:  []byte("hello"),
		})
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
	listener net.Listener
}

func newMockALPNServer(t *testing.T, supportedProtos []string) *mockALPNServer {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cert, err := tls.X509KeyPair(tlsCert, keyPEM)
	require.NoError(t, err)

	listener, err := tls.Listen("tcp", "localhost:0", &tls.Config{
		NextProtos:   supportedProtos,
		Certificates: []tls.Certificate{cert},
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		listener.Close()
	})

	go func() {
		t.Log("Mock ALPN server started.")
		defer t.Log("Mock ALPN server closed.")

		for {
			clientConn, err := listener.Accept()
			if errors.Is(err, net.ErrClosed) {
				break
			}

			go func() {
				clientConn.(*tls.Conn).HandshakeContext(ctx)
				clientConn.Close()
			}()
		}
	}()

	return &mockALPNServer{
		listener: listener,
	}
}

type mockConnUpgradeHandler struct {
	*require.Assertions

	WriteData []byte
}

func (h mockConnUpgradeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Equal(constants.ConnectionUpgradeWebAPI, r.URL.Path)
	h.Equal(constants.ConnectionUpgradeTypeALPN, r.Header.Get(constants.ConnectionUpgradeHeader))

	hj, ok := w.(http.Hijacker)
	h.True(ok)

	conn, _, err := hj.Hijack()
	h.NoError(err)
	defer conn.Close()

	response := &http.Response{
		StatusCode: http.StatusSwitchingProtocols,
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
	h.NoError(response.Write(conn))

	if len(h.WriteData) != 0 {
		conn.Write(h.WriteData)
	}
}
