/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tdpbv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/desktop/v1"
	tdp "github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/legacy"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp/protocol/tdpb"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	websocketBufferSize = 1024 * 16 // 16 KiB
)

func newWebsocketConn(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	serverConn := make(chan *websocket.Conn)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		defer close(serverConn)
		u := websocket.Upgrader{
			ReadBufferSize:  websocketBufferSize,
			WriteBufferSize: websocketBufferSize,
		}
		conn, err := u.Upgrade(w, r, nil)
		assert.NoError(t, err)
		serverConn <- conn
	}))

	clientconn, resp, err := websocket.DefaultDialer.Dial(strings.Replace(server.URL, "http", "ws", 1), nil)
	defer resp.Body.Close()

	require.NoError(t, err)

	t.Cleanup(server.Close)
	return clientconn, <-serverConn
}

func TestProxyConnection(t *testing.T) {
	// Echos back any TDPB messages received
	tdpbEchoServer := func(conn net.Conn) error {
		rdr := bufio.NewReader(conn)
		for {
			msg, err := tdpb.DecodeStrict(rdr)
			if err != nil {
				return err
			}

			if err = tdp.EncodeTo(conn, msg); err != nil {
				return err
			}
		}
	}

	// Echos back any TDP messages received
	tdpEchoServer := func(conn net.Conn) error {
		tdpConn := tdp.NewConn(conn, legacy.Decode)
		for {
			msg, err := tdpConn.ReadMessage()
			if err != nil {
				return err
			}

			if err = tdpConn.WriteMessage(msg); err != nil {
				return err
			}
		}
	}

	tdpClient := func(w *websocket.Conn, expectLatency bool) error {
		conn := tdp.NewConn(&WebsocketIO{Conn: w}, legacy.Decode)

		rdpMsg := legacy.RDPResponsePDU([]byte("hello"))
		mouseMsg := legacy.MouseWheel{Axis: 2, Delta: 4}

		if err := conn.WriteMessage(rdpMsg); err != nil {
			return err
		}
		if err := conn.WriteMessage(mouseMsg); err != nil {
			return err
		}

		// Read until we get these messages back
		gotRDP := false
		gotMouse := false
		gotLatency := !expectLatency
		for !gotRDP || !gotMouse || !gotLatency {
			msg, err := conn.ReadMessage()
			if err != nil {
				return err
			}

			switch m := msg.(type) {
			case legacy.RDPResponsePDU:
				assert.Equal(t, rdpMsg, m)
				gotRDP = true
			case legacy.MouseWheel:
				assert.Equal(t, mouseMsg, m)
				gotMouse = true
			case legacy.LatencyStats:
				if !expectLatency {
					t.Error("unexpected latency stats")
				}
				gotLatency = true
			default:
				return fmt.Errorf("received unexpected message type %T", m)
			}
		}
		return nil
	}

	tdpbClient := func(w *websocket.Conn, expectLatency bool) error {
		conn := tdp.NewConn(&WebsocketIO{Conn: w}, tdp.DecoderAdapter(tdpb.DecodePermissive))

		rdpMsg := &tdpb.RDPResponsePDU{
			Response: []byte("hello"),
		}

		mouseMsg := &tdpb.MouseWheel{Axis: tdpbv1.MouseWheelAxis_MOUSE_WHEEL_AXIS_HORIZONTAL, Delta: 4}

		if err := conn.WriteMessage(rdpMsg); err != nil {
			return err
		}
		if err := conn.WriteMessage(mouseMsg); err != nil {
			return err
		}

		// Read until we get these messages back
		gotRDP := false
		gotMouse := false
		gotLatency := !expectLatency
		for !gotRDP || !gotMouse || !gotLatency {
			msg, err := conn.ReadMessage()
			if err != nil {
				return err
			}

			switch m := msg.(type) {
			case *tdpb.RDPResponsePDU:
				//assert.Truef(t, proto.Equal(m, rdpMsg), "got: %+v, expected: %+v", m, rdpMsg)
				assert.True(t, bytes.Equal(rdpMsg.Response, m.Response))
				gotRDP = true
			case *tdpb.MouseWheel:
				//assert.Truef(t, proto.Equal(m, mouseMsg), "got: %+v, expected: %+v", m, mouseMsg)
				assert.Equal(t, m.Axis, mouseMsg.Axis)
				assert.Equal(t, m.Delta, mouseMsg.Delta)
				gotMouse = true
			case *tdpb.LatencyStats:
				if !expectLatency {
					t.Error("unexpected latency stats")
				}
				gotLatency = true
			default:
				return fmt.Errorf("received unexpected message type %T", m)
			}
		}
		return nil
	}

	tests := []struct {
		name           string
		clientProtocol string
		serverProtocol string
		version        string
		echoFn         func(net.Conn) error
		clientFn       func(*websocket.Conn, bool) error
	}{
		{
			name:           "tdp-tdpb",
			clientProtocol: protocolTDP,
			serverProtocol: tdpb.ProtocolName,
			version:        "17.5.0",
			clientFn:       tdpClient,
			echoFn:         tdpbEchoServer,
		},
		{
			name:           "tdp-tdp",
			clientProtocol: protocolTDP,
			serverProtocol: protocolTDP,
			version:        "17.5.0",
			clientFn:       tdpClient,
			echoFn:         tdpEchoServer,
		},
		{
			name:           "tdpb-tdp",
			clientProtocol: tdpb.ProtocolName,
			serverProtocol: protocolTDP,
			version:        "17.5.0",
			clientFn:       tdpbClient,
			echoFn:         tdpEchoServer,
		},
		{
			name:           "tdpb-tdpb",
			clientProtocol: tdpb.ProtocolName,
			serverProtocol: tdpb.ProtocolName,
			version:        "17.5.0",
			clientFn:       tdpbClient,
			echoFn:         tdpbEchoServer,
		},
		{
			name:           "tdp-tdpb-no-latency-monitor",
			clientProtocol: protocolTDP,
			serverProtocol: tdpb.ProtocolName,
			/* server version does not support latency monitoring */
			version:  "17.0.0",
			clientFn: tdpClient,
			echoFn:   tdpbEchoServer,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// We'll name the connection handles "inner" and "outter".
			// The proxy owns in "inner" handles while our test harness works with "outter" handles.
			// Outer Client Handle <---> Proxy(Inner Client Handle, Inner Server Handle) <---> Outer Server Handle
			serverIn, serverOut := net.Pipe()
			clientIn, clientOut := newWebsocketConn(t)

			wg := sync.WaitGroup{}
			var proxyErr, echoErr error
			wg.Go(func() {
				proxyErr = desktopWebsocketProxy{clientIn, serverIn, test.version, test.clientProtocol, test.serverProtocol, slog.Default()}.run(t.Context())
			})
			wg.Go(func() {
				echoErr = test.echoFn(serverOut)
			})

			latencySupported, _ := utils.MinVerWithoutPreRelease(test.version, "17.5.0")
			require.NoError(t, test.clientFn(clientOut, latencySupported))
			// Handle websocket shutdown
			require.NoError(t, clientOut.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Shutting down gracefully"), time.Now().Add(2*time.Second)))
			_, _, err := clientOut.ReadMessage()
			var wsErr *websocket.CloseError
			require.ErrorAs(t, err, &wsErr)
			require.Equal(t, websocket.CloseNormalClosure, wsErr.Code)

			wg.Wait()
			require.ErrorIs(t, echoErr, io.EOF)
			require.ErrorAs(t, proxyErr, &wsErr)
			require.Equal(t, websocket.CloseNormalClosure, wsErr.Code)
			_ = serverOut.Close()
		})
	}
}

func TestHandshaker(t *testing.T) {

	t.Run("tdp-client", func(t *testing.T) {
		handshaker, client := newWebsocketConn(t)
		clientConn := WebsocketIO{Conn: client}
		defer handshaker.Close()
		defer client.Close()

		shaker := newHandshaker(protocolTDP, handshaker)

		done := make(chan error)
		go func() {
			done <- shaker.performInitialHandshake(t.Context(), slog.Default())
		}()

		// Send two Screenspecs
		spec := legacy.ClientScreenSpec{Width: 1, Height: 2}
		require.NoError(t, tdp.EncodeTo(&clientConn, spec))
		require.NoError(t, tdp.EncodeTo(&clientConn, spec))
		// Should succeed
		require.NoError(t, <-done)

		buf := bytes.NewBuffer(nil)
		// Ask for keyboard layout, though we won't get one
		require.NoError(t, shaker.forwardTDP(buf, "bob", true))

		msg, err := legacy.Decode(buf)
		require.NoError(t, err)
		require.IsType(t, legacy.ClientUsername{}, msg)
		assert.Equal(t, "bob", msg.(legacy.ClientUsername).Username)

		// Should get two ClientScreenSpecs as well
		// One is explicitly forwarded, once was withheld while
		// anticipating the KeyboardLayout that never arrived.
		for range 2 {
			msg, err := legacy.Decode(buf)
			require.NoError(t, err)
			require.IsType(t, legacy.ClientScreenSpec{}, msg)
		}
		// Should be empty
		_, err = legacy.Decode(buf)
		require.ErrorIs(t, err, io.EOF)

		// Now test forwarding as TDPB
		// Ask for keyboard layout, though we won't get one
		require.NoError(t, shaker.forwardTDPB(buf, "bob", true))

		msg, err = tdpb.DecodeStrict(buf)
		require.NoError(t, err)
		require.IsType(t, &tdpb.ClientHello{}, msg)
		// Make sure everything got translated correctly.
		assert.Equal(t, "bob", msg.(*tdpb.ClientHello).Username)
		assert.Equal(t, uint32(1), msg.(*tdpb.ClientHello).ScreenSpec.Width)
		assert.Equal(t, uint32(2), msg.(*tdpb.ClientHello).ScreenSpec.Height)
	})

	t.Run("tdpb-client", func(t *testing.T) {
		handshaker, client := newWebsocketConn(t)
		clientConn := WebsocketIO{Conn: client}
		defer handshaker.Close()
		defer client.Close()

		shaker := newHandshaker(tdpb.ProtocolName, handshaker)

		done := make(chan error)
		go func() {
			done <- shaker.performInitialHandshake(t.Context(), slog.Default())
		}()

		// Send Client Hello plus a random message to be withheld
		hello := &tdpb.ClientHello{
			ScreenSpec: &tdpbv1.ClientScreenSpec{
				Width:  1,
				Height: 2,
			},
			KeyboardLayout: 12,
		}
		require.NoError(t, tdp.EncodeTo(&clientConn, hello))
		// Should succeed
		require.NoError(t, <-done)

		buf := bytes.NewBuffer(nil)
		require.NoError(t, shaker.forwardTDPB(buf, "bob", false /* should be ignored */))

		msg, err := tdpb.DecodeStrict(buf)
		require.NoError(t, err)
		// Should get the Client Hello back
		require.IsType(t, &tdpb.ClientHello{}, msg)
		assert.Equal(t, "bob", msg.(*tdpb.ClientHello).Username)
		assert.Equal(t, uint32(1), msg.(*tdpb.ClientHello).ScreenSpec.Width)
		assert.Equal(t, uint32(2), msg.(*tdpb.ClientHello).ScreenSpec.Height)
		assert.Equal(t, uint32(12), msg.(*tdpb.ClientHello).KeyboardLayout)

		// Should be empty
		_, err = tdpb.DecodeStrict(buf)
		require.ErrorIs(t, err, io.EOF)

		// Now test forwarding as TDP
		require.NoError(t, shaker.forwardTDP(buf, "bob", true))

		msg, err = legacy.Decode(buf)
		require.NoError(t, err)
		require.IsType(t, legacy.ClientUsername{}, msg)
		assert.Equal(t, "bob", msg.(legacy.ClientUsername).Username)

		msg, err = legacy.Decode(buf)
		require.NoError(t, err)
		require.IsType(t, legacy.ClientScreenSpec{}, msg)
		assert.Equal(t, uint32(1), msg.(legacy.ClientScreenSpec).Width)
		assert.Equal(t, uint32(2), msg.(legacy.ClientScreenSpec).Height)

		msg, err = legacy.Decode(buf)
		require.NoError(t, err)
		require.IsType(t, legacy.ClientKeyboardLayout{}, msg)
		assert.Equal(t, uint32(12), msg.(legacy.ClientKeyboardLayout).KeyboardLayout)

		// Should be empty
		_, err = legacy.Decode(buf)
		require.ErrorIs(t, err, io.EOF)
	})

	t.Run("withheld-tdpb-messages-are-translated", func(t *testing.T) {
		tdpbHandshaker := tdpbHandshaker{
			hello: &tdpb.ClientHello{
				ScreenSpec: &tdpbv1.ClientScreenSpec{
					Width:  10,
					Height: 10,
				},
				KeyboardLayout: 1,
			},
			withheld: []tdp.Message{&tdpb.MouseMove{X: 1, Y: 2}},
		}

		buf := bytes.NewBuffer(nil)
		require.NoError(t, tdpbHandshaker.forwardTDP(buf, "someuser", false))
		username, err := legacy.Decode(buf)
		require.IsType(t, legacy.ClientUsername{}, username)
		require.NoError(t, err)
		screenSpec, err := legacy.Decode(buf)
		require.NoError(t, err)
		require.IsType(t, legacy.ClientScreenSpec{}, screenSpec)
		pngFrame, err := legacy.Decode(buf)
		require.NoError(t, err)
		require.IsType(t, legacy.MouseMove{}, pngFrame)
	})

	t.Run("withheld-tdp-messages-are-translated", func(t *testing.T) {
		tdbHandshaker := tdpHandshaker{
			screenSpec: legacy.ClientScreenSpec{
				Width:  10,
				Height: 10,
			},
			withheld: []tdp.Message{legacy.MouseMove{X: 1, Y: 2}},
		}

		buf := bytes.NewBuffer(nil)
		require.NoError(t, tdbHandshaker.forwardTDPB(buf, "someuser", false))
		hello, err := tdpb.DecodePermissive(buf)
		require.IsType(t, &tdpb.ClientHello{}, hello)
		require.NoError(t, err)
		pngFrame, err := tdpb.DecodePermissive(buf)
		require.NoError(t, err)
		require.IsType(t, &tdpb.MouseMove{}, pngFrame)
	})
}

func TestDesktopWebsocketAdapter(t *testing.T) {
	test, adapted := newWebsocketConn(t)
	defer test.Close()
	defer adapted.Close()

	adapter := &desktopWebsocketAdapter{
		conn: adapted,
	}

	tdpAlert, err := legacy.Alert{Message: "tdp!", Severity: legacy.SeverityWarning}.Encode()
	require.NoError(t, err)

	tdpbAlert, err := (&tdpb.Alert{Message: "tdpb!", Severity: tdpbv1.AlertSeverity_ALERT_SEVERITY_WARNING}).Encode()
	require.NoError(t, err)
	// Send a TDP message followed by TDPB
	require.NoError(t, test.WriteMessage(websocket.BinaryMessage, tdpAlert))
	require.NoError(t, test.WriteMessage(websocket.BinaryMessage, tdpbAlert))

	// TDP message should be discarded and only the TDPB
	// alert should be read from the websocket.
	msg, err := adapter.ReadMessage()
	require.NoError(t, err)
	require.IsType(t, &tdpb.Alert{}, msg)
	assert.Equal(t, "tdpb!", msg.(*tdpb.Alert).Message)
}
