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
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/gobwas/ws"
	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/constants"
)

func TestClient(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name       string
		protocols  []string
		serverTest func(*testing.T) http.HandlerFunc
	}
	testCases := []testCase{
		{
			name:      "WebSocket with close",
			protocols: []string{constants.WebAPIConnUpgradeProtocolWebSocketClose},
			serverTest: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					conn, _, hs, err := wsProtocolUpgrader.Upgrade(r, w)
					assert.NoError(t, err, "Failed to upgrade WebSocket connection")
					defer conn.Close()
					assert.Equal(t, constants.WebAPIConnUpgradeProtocolWebSocketClose, hs.Protocol, "Handshake protocol should match ALPN")

					frame, err := ws.ReadFrame(conn)
					assert.NoError(t, err, "Failed to read WebSocket frame")
					assert.Equal(t, ws.OpBinary, frame.Header.OpCode, "Expected text frame")
					assert.Equal(t, payload, string(frame.Payload), "Payload should match expected value")

					// Write a response frame to the client.
					err = ws.WriteFrame(conn, ws.NewFrame(ws.OpBinary, false, []byte(payload)))
					assert.NoError(t, err, "Failed to write WebSocket frame")

					// Write a close frame to the client.
					err = ws.WriteFrame(conn, ws.NewCloseFrame(ws.NewCloseFrameBody(ws.StatusNormalClosure, "")))
					assert.NoError(t, err, "Failed to write WebSocket close frame")

					// Read the close frame from the client.
					frame, err = ws.ReadFrame(conn)
					assert.NoError(t, err, "Failed to read WebSocket close frame")
					assert.Equal(t, ws.OpClose, frame.Header.OpCode, "Expected close frame")
					code, message := ws.ParseCloseFrameData(frame.Payload)
					assert.Equal(t, ws.StatusNormalClosure, code, "Expected normal closure status")
					assert.Empty(t, message, "Expected no close message")
				}
			},
		},
		{
			name:      "WebSocket with close terminated by Close call",
			protocols: []string{constants.WebAPIConnUpgradeProtocolWebSocketClose},
			serverTest: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					conn, _, hs, err := wsProtocolUpgrader.Upgrade(r, w)
					assert.NoError(t, err, "Failed to upgrade WebSocket connection")

					assert.Equal(t, constants.WebAPIConnUpgradeProtocolWebSocketClose, hs.Protocol, "Handshake protocol should match ALPN")

					frame, err := ws.ReadFrame(conn)
					assert.NoError(t, err, "Failed to read WebSocket frame")
					assert.Equal(t, ws.OpBinary, frame.Header.OpCode, "Expected text frame")
					assert.Equal(t, payload, string(frame.Payload), "Payload should match expected value")

					// Write a response frame to the client.
					err = ws.WriteFrame(conn, ws.NewFrame(ws.OpBinary, false, []byte(payload)))
					assert.NoError(t, err, "Failed to write WebSocket frame")

					// Write a close frame to the client.
					err = ws.WriteFrame(conn, ws.NewCloseFrame(ws.NewCloseFrameBody(ws.StatusNormalClosure, "")))
					assert.NoError(t, err, "Failed to write WebSocket close frame")

					conn.Close()
				}
			},
		},
		{
			name:      "WebSocket without close",
			protocols: []string{constants.WebAPIConnUpgradeTypeALPN},
			serverTest: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					conn, _, hs, err := wsProtocolUpgrader.Upgrade(r, w)
					assert.NoError(t, err, "Failed to upgrade WebSocket connection")
					defer conn.Close()
					assert.Equal(t, constants.WebAPIConnUpgradeTypeALPN, hs.Protocol, "Handshake protocol should match ALPN")

					frame, err := ws.ReadFrame(conn)
					assert.NoError(t, err, "Failed to read WebSocket frame")
					assert.Equal(t, ws.OpBinary, frame.Header.OpCode, "Expected text frame")
					assert.Equal(t, payload, string(frame.Payload), "Payload should match expected value")

					// Simulate a close without sending a close frame.
					// This should still read the close frame sent by the server
					// and the server should close the connection after not receiving a close frame.
					err = ws.WriteFrame(conn, ws.NewFrame(ws.OpBinary, false, []byte(payload)))
					assert.NoError(t, err, "Failed to write WebSocket frame")
				}
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := createHTTPServer(t, tc.serverTest(t))
			t.Cleanup(server.Close)

			ctx := context.Background()
			u, err := url.Parse(server.URL)
			assert.NoError(t, err, "Failed to parse server URL")
			conn, err := NewWebSocketALPNClientConn(ctx,
				WebSocketALPNClientConnConfig{
					URL:       u,
					Dialer:    (&net.Dialer{}).DialContext,
					TLSConfig: tlsConfigForHTTPServer(t, server),
					Protocols: tc.protocols,
					Logger:    slog.Default(),
				},
			)
			assert.NoError(t, err, "Failed to create WebSocket ALPN client connection")
			t.Cleanup(func() {
				conn.Close()
			})

			_, err = conn.Write([]byte(payload))
			assert.NoError(t, err, "Failed to write payload to WebSocket connection")

			// Read the response from the server.
			var data [1024]byte
			n, err := conn.Read(data[:])
			assert.NoError(t, err, "Failed to read from WebSocket connection")
			assert.Equal(t, payload, string(data[:n]), "Response payload should match expected value")

			// Verify the connection is closed gracefully.
			_, err = conn.Read(data[:])
			assert.True(t, errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF), "Expected connection to be closed")
		})
	}
}
