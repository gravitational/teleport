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

package web_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
)

// TestTerminalReadFromClosedConn verifies that Teleport recovers
// from a closed websocket connection.
// See https://github.com/gravitational/teleport/issues/21334
func TestTerminalReadFromClosedConn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var upgrader = websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("couldn't upgrade websocket connection: %v", err)
		}

		envelope := web.Envelope{
			Type:    defaults.WebsocketRaw,
			Payload: "hello",
		}
		b, err := proto.Marshal(&envelope)
		if err != nil {
			t.Errorf("could not marshal envelope: %v", err)
		}
		conn.WriteMessage(websocket.BinaryMessage, b)
	}))
	t.Cleanup(server.Close)

	u := strings.Replace(server.URL, "http:", "ws:", 1)
	conn, resp, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	stream := web.NewTerminalStream(context.Background(), conn, utils.NewLoggerForTests())

	// close the stream before we attempt to read from it,
	// this will produce a net.ErrClosed error on the read
	require.NoError(t, stream.Close())

	_, err = io.Copy(io.Discard, stream)
	require.NoError(t, err)
}
