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

package streamproto

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

var upgrader = websocket.Upgrader{}

func TestPingPong(t *testing.T) {
	t.Parallel()

	runClient := func(conn *websocket.Conn) error {
		client, err := NewSessionStream(conn, ClientHandshake{Mode: types.SessionPeerMode})
		if err != nil {
			return trace.Wrap(err)
		}

		n, err := client.Write([]byte("ping"))
		if err != nil {
			return trace.Wrap(err)
		}
		if n != 4 {
			return trace.Errorf("unexpected write size: %d", n)
		}

		out := make([]byte, 4)
		_, err = io.ReadFull(client, out)
		if err != nil {
			return trace.Wrap(err)
		}
		if string(out) != "pong" {
			return trace.BadParameter("expected pong, got %q", out)
		}

		return nil
	}

	runServer := func(conn *websocket.Conn) error {
		server, err := NewSessionStream(conn, ServerHandshake{MFARequired: false})
		if err != nil {
			return trace.Wrap(err)
		}

		out := make([]byte, 4)
		_, err = io.ReadFull(server, out)
		if err != nil {
			return trace.Wrap(err)
		}
		if string(out) != "ping" {
			return trace.BadParameter("expected ping, got %q", out)
		}

		n, err := server.Write([]byte("pong"))
		if err != nil {
			return trace.Wrap(err)
		}
		if n != 4 {
			return trace.Errorf("unexpected write size: %d", n)
		}

		return nil
	}

	errCh := make(chan error, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		defer ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{})
		errCh <- runServer(ws)
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, resp, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)

	// Always drain/close the body.
	io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	go func() {
		defer ws.Close()
		errCh <- runClient(ws)
	}()

	require.NoError(t, <-errCh)
	require.NoError(t, <-errCh)
}

// TestReadTaskDoubleForceTerminatePanics reproduces the unguarded
// close(s.forceTerminate) in SessionStream.readTask. A peer that sends two
// {"force_terminate":true} text frames double-closes the channel, panicking
// the read goroutine ("close of closed channel"). The goroutine has no
// deferred recover, so on unpatched code the Go runtime terminates the
// entire test binary. On patched code (sync.Once or atomic.Bool guard) the
// second frame is a no-op and the test completes cleanly.
func TestReadTaskDoubleForceTerminatePanics(t *testing.T) {
	forceFrame, err := utils.FastMarshal(metaMessage{ForceTerminate: true})
	require.NoError(t, err)

	clientHandshakeFrame, err := utils.FastMarshal(metaMessage{
		ClientHandshake: &ClientHandshake{Mode: types.SessionObserverMode},
	})
	require.NoError(t, err)

	serverDone := make(chan error, 1)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			serverDone <- trace.Wrap(err)
			return
		}
		defer ws.Close()

		server, err := NewSessionStream(ws, ServerHandshake{MFARequired: false})
		if err != nil {
			serverDone <- trace.Wrap(err)
			return
		}

		// Wait for the first force_terminate to come through.
		select {
		case <-server.ForceTerminateQueue():
		case <-time.After(2 * time.Second):
			serverDone <- trace.Errorf("server timed out waiting for ForceTerminate signal")
			return
		}

		// Give the readTask goroutine time to process the second
		// force_terminate frame. On unpatched code this is where the
		// runtime panic fires and brings the binary down.
		time.Sleep(200 * time.Millisecond)
		serverDone <- nil
	}))
	t.Cleanup(httpSrv.Close)

	url := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	ws, resp, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	// Read the server handshake that NewSessionStream writes first.
	typ, _, err := ws.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, typ)

	// Send a valid client handshake. Any participant mode passes the
	// protocol-level handshake; CanJoin lives a layer up and is not
	// exercised by streamproto on its own.
	require.NoError(t, ws.WriteMessage(websocket.TextMessage, clientHandshakeFrame))

	// First force_terminate — closes the channel cleanly.
	require.NoError(t, ws.WriteMessage(websocket.TextMessage, forceFrame))

	// Second force_terminate — on unpatched code this triggers
	// "close of closed channel" panic in readTask.
	require.NoError(t, ws.WriteMessage(websocket.TextMessage, forceFrame))

	select {
	case err := <-serverDone:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("server handler did not complete; readTask likely panicked")
	}
}
