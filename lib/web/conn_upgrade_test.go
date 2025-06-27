/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package web

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/pingconn"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/listener"
)

func TestWriteUpgradeResponse(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, writeUpgradeResponse(&buf, "custom"))

	resp, err := http.ReadResponse(bufio.NewReader(&buf), nil)
	require.NoError(t, err)

	// Always drain/close the body.
	io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	require.Equal(t, "custom", resp.Header.Get("Upgrade"))
}

func TestHandlerConnectionUpgrade(t *testing.T) {
	expectedPayload := "hello@"
	expectedIP := "1.2.3.4"
	simpleWriteHandler := func(t *testing.T) ConnectionHandler {
		t.Helper()
		return func(_ context.Context, conn net.Conn) error {
			// Handles connection asynchronously to verify web handler waits until
			// connection is closed.
			go func() {
				defer conn.Close()

				clientIP, err := utils.ClientIPFromConn(conn)
				require.NoError(t, err)
				require.Equal(t, expectedIP, clientIP)

				n, err := conn.Write([]byte(expectedPayload))
				require.NoError(t, err)
				require.Len(t, expectedPayload, n)
			}()
			return nil
		}
	}

	nestedUpgradeHandler := func(t *testing.T) ConnectionHandler {
		t.Helper()
		return func(ctx context.Context, conn net.Conn) error {
			connListener := listener.NewSingleUseListener(conn)
			http.Serve(connListener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer conn.Close()
				upgrader := websocket.Upgrader{}
				wsConn, err := upgrader.Upgrade(w, r, nil)
				require.NoError(t, err)

				op, payload, err := wsConn.ReadMessage()
				require.NoError(t, err)
				require.Equal(t, websocket.BinaryMessage, op)
				require.Equal(t, expectedPayload, string(payload))
			}))
			return nil
		}
	}

	tests := []struct {
		name                  string
		inputALPNHandler      func(*testing.T) ConnectionHandler
		inputRequest          *http.Request
		expectUpgradeType     string
		checkHandlerError     func(error) bool
		wrapClientConn        func(net.Conn) net.Conn
		checkClientConnString func(*testing.T, net.Conn, string)
	}{
		{
			name:              "unsupported type",
			inputALPNHandler:  simpleWriteHandler,
			inputRequest:      makeConnUpgradeRequest(t, "", "unsupported-protocol", expectedIP),
			checkHandlerError: trace.IsNotFound,
		},
		{
			// TODO(greedy52) DELETE in 17.0
			name:                  "upgraded to ALPN (legacy)",
			inputALPNHandler:      simpleWriteHandler,
			inputRequest:          makeConnUpgradeRequest(t, "", constants.WebAPIConnUpgradeTypeALPN, expectedIP),
			expectUpgradeType:     constants.WebAPIConnUpgradeTypeALPN,
			checkClientConnString: mustReadClientConnString,
		},
		{
			// TODO(greedy52) DELETE in 17.0
			name:                  "upgraded to ALPN with Ping (legacy)",
			inputALPNHandler:      simpleWriteHandler,
			inputRequest:          makeConnUpgradeRequest(t, "", constants.WebAPIConnUpgradeTypeALPNPing, expectedIP),
			expectUpgradeType:     constants.WebAPIConnUpgradeTypeALPNPing,
			wrapClientConn:        toNetConn(pingconn.New),
			checkClientConnString: mustReadClientConnString,
		},
		{
			// TODO(greedy52) DELETE in 17.0
			name:                  "nested ALPN (legacy) upgrade",
			inputALPNHandler:      nestedUpgradeHandler,
			inputRequest:          makeConnUpgradeRequest(t, "", constants.WebAPIConnUpgradeTypeALPN, expectedIP),
			expectUpgradeType:     constants.WebAPIConnUpgradeTypeALPN,
			checkClientConnString: mustWriteNestedWebSocketConnString,
		},
		{
			name:                  "upgraded to ALPN with Teleport-specific header",
			inputALPNHandler:      simpleWriteHandler,
			inputRequest:          makeConnUpgradeRequest(t, constants.WebAPIConnUpgradeTeleportHeader, constants.WebAPIConnUpgradeTypeALPN, expectedIP),
			expectUpgradeType:     constants.WebAPIConnUpgradeTypeALPN,
			checkClientConnString: mustReadClientConnString,
		},
		{
			name:                  "upgraded to WebSocket",
			inputALPNHandler:      simpleWriteHandler,
			inputRequest:          makeConnUpgradeWebSocketRequest(t, constants.WebAPIConnUpgradeTypeALPN, expectedIP),
			expectUpgradeType:     constants.WebAPIConnUpgradeTypeWebSocket,
			wrapClientConn:        toNetConn(newWebsocketALPNClientConn),
			checkClientConnString: mustReadClientConnString,
		},
		{
			name:                  "upgraded to WebSocket with ping",
			inputALPNHandler:      simpleWriteHandler,
			inputRequest:          makeConnUpgradeWebSocketRequest(t, constants.WebAPIConnUpgradeTypeALPNPing, expectedIP),
			expectUpgradeType:     constants.WebAPIConnUpgradeTypeWebSocket,
			wrapClientConn:        toNetConn(newWebsocketALPNClientConn),
			checkClientConnString: mustReadClientConnString,
		},
		{
			name:                  "unsupported WebSocket sub-protocol",
			inputALPNHandler:      simpleWriteHandler,
			inputRequest:          makeConnUpgradeWebSocketRequest(t, "unsupported-protocol", expectedIP),
			expectUpgradeType:     constants.WebAPIConnUpgradeTypeWebSocket,
			checkClientConnString: mustReadClientWebSocketClosed,
		},
		{
			name:                  "nested WebSocket upgrade",
			inputALPNHandler:      nestedUpgradeHandler,
			inputRequest:          makeConnUpgradeWebSocketRequest(t, constants.WebAPIConnUpgradeTypeALPN, expectedIP),
			expectUpgradeType:     constants.WebAPIConnUpgradeTypeWebSocket,
			wrapClientConn:        toNetConn(newWebsocketALPNClientConn),
			checkClientConnString: mustWriteNestedWebSocketConnString,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Cherry picked some attributes to create a Handler to test only the
			// connection upgrade portion.
			h := &Handler{
				cfg: Config{
					ALPNHandler: test.inputALPNHandler(t),
				},
				logger: slog.Default(),
				clock:  clockwork.NewRealClock(),
			}

			serverConn, clientConn := net.Pipe()
			defer serverConn.Close()
			defer clientConn.Close()

			// serverConn will be hijacked.
			w := newResponseWriterHijacker(nil, serverConn)

			// Serve the handler with XForwardedFor middleware to set IPs.
			handlerErrChan := make(chan error, 1)
			go func() {
				connUpgradeHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := h.connectionUpgrade(w, r, nil)
					handlerErrChan <- err
				})

				NewXForwardedForMiddleware(connUpgradeHandler).ServeHTTP(w, test.inputRequest)
			}()

			select {
			case handlerErr := <-handlerErrChan:
				if test.checkHandlerError != nil {
					require.Error(t, handlerErr)
					require.True(t, test.checkHandlerError(handlerErr))
				} else {
					require.NoError(t, handlerErr)
				}

			case <-w.hijackedCtx.Done():
				mustReadSwitchProtocolsResponse(t, test.inputRequest, clientConn, test.expectUpgradeType)
				if test.wrapClientConn != nil {
					clientConn = test.wrapClientConn(clientConn)
				}
				test.checkClientConnString(t, clientConn, expectedPayload)

			case <-time.After(5 * time.Second):
				require.Fail(t, "timed out waiting for handler to serve")
			}
		})
	}
}

func toNetConn[ConnType net.Conn](f func(net.Conn) ConnType) func(net.Conn) net.Conn {
	return func(in net.Conn) net.Conn {
		return net.Conn(f(in))
	}
}

// websocketALPNClientConn wraps the provided net.Conn after WebSocket
// handshake (101 switch) is performed. This is a simpler version of the client
// connection wrapper in api/client.
type websocketALPNClientConn struct {
	net.Conn
}

func newWebsocketALPNClientConn(conn net.Conn) *websocketALPNClientConn {
	return &websocketALPNClientConn{conn}
}

func (c *websocketALPNClientConn) Read(b []byte) (int, error) {
	frame, err := ws.ReadFrame(c.Conn)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	switch frame.Header.OpCode {
	case ws.OpClose:
		return 0, io.EOF
	case ws.OpBinary:
		return copy(b, frame.Payload), nil
	case ws.OpPing:
		return c.Read(b)
	default:
		return 0, trace.BadParameter("unsupported op %v", frame.Header.OpCode)
	}
}

func (c *websocketALPNClientConn) Write(b []byte) (int, error) {
	// Remember to use a proper mask on client side.
	frame := ws.MaskFrame(ws.NewFrame(ws.OpBinary, true, b))
	if err := ws.WriteFrame(c.Conn, frame); err != nil {
		return 0, trace.Wrap(err)
	}
	return len(b), nil
}

func mustWriteNestedWebSocketConnString(t *testing.T, clientConn net.Conn, payload string) {
	t.Helper()

	dialer := websocket.Dialer{
		NetDialContext: func(context.Context, string, string) (net.Conn, error) {
			return clientConn, nil
		},
	}
	nestedConn, response, err := dialer.DialContext(context.Background(), "ws://does-not-matter", nil)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	require.NoError(t, err)
	require.NoError(t, nestedConn.WriteMessage(websocket.BinaryMessage, []byte(payload)))
}

func makeConnUpgradeRequest(t *testing.T, upgradeHeaderKey, upgradeType, xForwardedFor string) *http.Request {
	t.Helper()

	if upgradeHeaderKey == "" {
		upgradeHeaderKey = constants.WebAPIConnUpgradeHeader
	}

	r, err := http.NewRequest("GET", "http://localhost/webapi/connectionupgrade", nil)
	require.NoError(t, err)
	r.Header.Add(upgradeHeaderKey, upgradeType)
	r.Header.Add("X-Forwarded-For", xForwardedFor)
	return r
}

func makeConnUpgradeWebSocketRequest(t *testing.T, alpnUpgradeType, xForwardedFor string) *http.Request {
	t.Helper()

	r, err := http.NewRequest("GET", "http://localhost/webapi/connectionupgrade", nil)
	require.NoError(t, err)

	// Append "legacy" upgrade. This tests whether the handler prefers "websocket".
	r.Header.Add(constants.WebAPIConnUpgradeHeader, alpnUpgradeType)
	r.Header.Add("X-Forwarded-For", xForwardedFor)

	// Add WebSocket headers
	r.Header.Add(constants.WebAPIConnUpgradeHeader, "websocket")
	r.Header.Add(constants.WebAPIConnUpgradeConnectionHeader, "upgrade")
	r.Header.Set("Sec-Websocket-Protocol", alpnUpgradeType)
	r.Header.Set("Sec-Websocket-Version", "13")
	r.Header.Set("Sec-Websocket-Key", "MTIzNDU2Nzg5MDEyMzQ1Ng==")
	return r
}

func mustReadSwitchProtocolsResponse(t *testing.T, r *http.Request, clientConn net.Conn, upgradeType string) {
	t.Helper()

	resp, err := http.ReadResponse(bufio.NewReader(clientConn), r)
	require.NoError(t, err)

	// Always drain/close the body.
	io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if upgradeType != "websocket" {
		require.Equal(t, upgradeType, resp.Header.Get(constants.WebAPIConnUpgradeTeleportHeader))
	}
	require.Equal(t, upgradeType, resp.Header.Get(constants.WebAPIConnUpgradeHeader))
	require.Equal(t, constants.WebAPIConnUpgradeConnectionType, resp.Header.Get(constants.WebAPIConnUpgradeConnectionHeader))
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
}

func mustReadClientConnString(t *testing.T, clientConn net.Conn, expectedPayload string) {
	t.Helper()

	receive, err := bufio.NewReader(clientConn).ReadString(byte('@'))
	require.NoError(t, err)
	require.Equal(t, expectedPayload, receive)
}

func mustReadClientWebSocketClosed(t *testing.T, clientConn net.Conn, expectedPayload string) {
	t.Helper()

	_, err := ws.ReadFrame(clientConn)
	require.True(t, utils.IsOKNetworkError(err))
}

// responseWriterHijacker is a mock http.ResponseWriter that also serves a
// net.Conn for http.Hijacker.
type responseWriterHijacker struct {
	http.ResponseWriter
	conn net.Conn

	// hijackedCtx is canceled when Hijack is called
	hijackedCtx       context.Context
	hijackedCtxCancel context.CancelFunc
}

func newResponseWriterHijacker(w http.ResponseWriter, conn net.Conn) *responseWriterHijacker {
	hijackedCtx, hijackedCtxCancel := context.WithCancel(context.Background())
	if w == nil {
		w = httptest.NewRecorder()
	}
	return &responseWriterHijacker{
		ResponseWriter:    w,
		conn:              conn,
		hijackedCtx:       hijackedCtx,
		hijackedCtxCancel: hijackedCtxCancel,
	}
}

func (h *responseWriterHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijackedCtxCancel()
	// buf is used by gorilla websocket upgrader.
	reader := bufio.NewReaderSize(nil, 10)
	writer := bufio.NewWriter(h.conn)
	buf := bufio.NewReadWriter(reader, writer)
	return h.conn, buf, nil
}
