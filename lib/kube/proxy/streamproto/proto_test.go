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
