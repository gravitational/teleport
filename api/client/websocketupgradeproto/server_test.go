/*
Copyright 2025 Gravitational, Inc.

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
package websocketupgradeproto

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gobwas/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
)

const payload = "test payload"

func TestServerProtocol(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name       string
		protocols  []string
		clientTest func(t *testing.T, conn net.Conn, hs ws.Handshake)
	}
	testCases := []testCase{
		{
			name:      "WebSocket without close",
			protocols: []string{constants.WebAPIConnUpgradeTypeALPN},
			clientTest: func(t *testing.T, conn net.Conn, hs ws.Handshake) {
				assert.Equal(t, constants.WebAPIConnUpgradeTypeALPN, hs.Protocol, "Handshake protocol should match ALPN")
				testClientConn(t, conn)
				var data [1024]byte

				frame, err := ws.ReadFrame(conn)
				assert.NoError(t, err, "Failed to read frame from connection")

				expectedFrame := ws.NewCloseFrame(ws.NewCloseFrameBody(ws.StatusNormalClosure, ""))
				assert.Equal(t, expectedFrame, frame, "Connection should receive close frame from server")
				assert.NoError(t, err, "Failed to read from connection")

				n, err := conn.Read(data[:])
				assert.Empty(t, data[:n], "Connection should not have any data after reading and not sending close")
				assert.ErrorIs(t, err, io.EOF, "Expected EOF after reading all data from connection")
			},
		},
		{
			name:      "WebSocket with close",
			protocols: []string{constants.WebAPIConnUpgradeProtocolWebSocketClose},
			clientTest: func(t *testing.T, conn net.Conn, hs ws.Handshake) {
				assert.Equal(t, constants.WebAPIConnUpgradeProtocolWebSocketClose, hs.Protocol, "Handshake protocol should match ALPN")
				testClientConn(t, conn)
				handleClose(t, conn)
				var data [1024]byte
				n, err := conn.Read(data[:])
				assert.Empty(t, data[:n], "Connection should not have any data after reading")
				assert.ErrorIs(t, err, io.EOF, "Expected EOF after reading all data from connection")
			},
		},
		{
			name:      "WebSocket with close without sending close",
			protocols: []string{constants.WebAPIConnUpgradeProtocolWebSocketClose},
			clientTest: func(t *testing.T, conn net.Conn, hs ws.Handshake) {
				assert.Equal(t, constants.WebAPIConnUpgradeProtocolWebSocketClose, hs.Protocol, "Handshake protocol should match ALPN")
				testClientConn(t, conn)
				// Simulate a close without sending a close frame.
				// This should still read the close frame sent by the server
				// and the server should close the connection after not receiving a close frame.
				payload, err := io.ReadAll(conn)
				assert.NoError(t, err, "Failed to read from connection")
				assert.Equal(t, payload, ws.CompiledCloseNormalClosure, "Payload should match expected value")
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := createHTTPServer(t, serverTestHandler(t))
			t.Cleanup(server.Close)

			ctx := context.Background()
			clientConn, hs := createClient(
				t,
				ctx,
				server,
				tc.protocols,
			)
			tc.clientTest(t, clientConn, hs)
		})
	}
}

func serverTestHandler(t *testing.T) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := NewServerConnection(
			nil,
			r,
			w,
		)
		defer func() {
			if conn != nil {
				_ = conn.Close()
			}
		}()
		assert.NoError(t, err, "Failed to create server connection")
		assert.NotNil(t, conn, "Server connection should not be nil")

		_, err = conn.Write([]byte(payload))
		assert.NoError(t, err, "Failed to write to server connection")

		// Read the message back to verify it was sent correctly.
		buf := make([]byte, len(payload))
		n, err := conn.Read(buf)
		assert.NoError(t, err, "Failed to read from server connection")
		assert.Equal(t, len(payload), n, "Read length should match written length")
		assert.Equal(t, payload, string(buf[:n]), "Read payload should match written payload")

		// Write a ping frame to test the ping functionality.
		err = conn.WritePing()
		assert.NoError(t, err, "Failed to write ping frame")
	},
	)
}

func createHTTPServer(t *testing.T, handler http.Handler) *httptest.Server {
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	return server
}

func createClient(t *testing.T, ctx context.Context, server *httptest.Server, protocols []string) (net.Conn, ws.Handshake) {
	dialer := ws.Dialer{
		Protocols: protocols,
		NetDial:   (&net.Dialer{}).DialContext,
		TLSConfig: tlsConfigForHTTPServer(t, server),
	}
	u, err := url.Parse(server.URL)
	require.NoError(t, err, "Failed to parse URL")
	u.Scheme = "wss"
	conn, _, hs, err := dialer.Dial(ctx, u.String())
	require.NoError(t, err, "Failed to dial WebSocket")
	t.Cleanup(func() {
		_ = conn.Close()
	})

	return conn, hs
}

func testClientConn(t *testing.T, conn net.Conn) {
	assert.NotNil(t, conn, "Client connection should not be nil")

	frame, err := ws.ReadFrame(conn)
	assert.NoError(t, err, "Failed to read frame from client connection")
	assert.Equal(t, ws.OpBinary, frame.Header.OpCode, "Frame should be a binary frame")
	assert.Equal(t, payload, string(frame.Payload), "Frame payload should match written payload")

	// Write a message to the server.
	err = ws.WriteFrame(conn, ws.NewBinaryFrame([]byte(payload)))
	assert.NoError(t, err, "Failed to write to client connection")

	// Read the PING frame back to verify the ping functionality.
	pingFrame, err := ws.ReadFrame(conn)
	assert.NoError(t, err, "Failed to read ping frame from client connection")
	assert.Equal(t, ws.OpPing, pingFrame.Header.OpCode, "Frame should be a ping frame")
	assert.Equal(t, ComponentTeleport, string(pingFrame.Payload), "Ping frame payload should match written payload")
}

func handleClose(t *testing.T, conn net.Conn) {
	// Wait for the server to send a close frame.
	closeFrame, err := ws.ReadFrame(conn)
	assert.NoError(t, err, "Failed to read close frame from client connection")
	assert.Equal(t, ws.OpClose, closeFrame.Header.OpCode, "Frame should be a close frame")
	code, message := ws.ParseCloseFrameData(closeFrame.Payload)
	assert.Equal(t, ws.StatusNormalClosure, code, "Close frame status code should be normal closure")
	assert.Empty(t, message, "Close frame message should be empty")

	// Close the connection with a normal closure status.
	err = ws.WriteFrame(conn, ws.NewCloseFrame(ws.NewCloseFrameBody(ws.StatusNormalClosure, "")))
	assert.NoError(t, err, "Failed to write close frame to client connection")
}

func TestClientUnsuportedProtocol(t *testing.T) {
	t.Parallel()

	server := createHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := NewServerConnection(nil, r, w)
		assert.Error(t, err, "Failed to create server connection")
		defer func() {
			if conn != nil {
				_ = conn.Close()
			}
		}()
	}))

	u, err := url.Parse(server.URL)
	require.NoError(t, err, "Failed to parse server URL")
	u.Scheme = "wss"
	dialer := ws.Dialer{
		Protocols: []string{"unsupported-protocol"},
		NetDial:   (&net.Dialer{}).DialContext,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	ctx := context.Background()
	conn, _, hs, err := dialer.Dial(ctx, u.String())
	require.NoError(t, err, "Failed to dial WebSocket")
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()

	assert.Empty(t, hs.Protocol, "Handshake protocol should be empty for unsupported protocol")
	frame, err := ws.ReadFrame(conn)
	assert.NoError(t, err, "Failed to read frame from client connection")
	assert.Equal(t, ws.OpClose, frame.Header.OpCode, "Frame should be a close frame")
	code, message := ws.ParseCloseFrameData(frame.Payload)
	assert.Equal(t, ws.StatusUnsupportedData, code, "Close frame status code should be unsupported data")
	assert.Equal(t, "unsupported WebSocket sub-protocol: unsupported-protocol", message, "Close frame message should match unsupported protocol error")
}
