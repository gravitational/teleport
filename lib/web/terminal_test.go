/*
Copyright 2023 Gravitational, Inc.

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
