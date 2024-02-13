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

package desktop_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/desktop"
)

func TestStreamsDesktopEvents(t *testing.T) {
	// set up a server that streams 4 events over websocket
	events := []apievents.AuditEvent{
		&apievents.DesktopRecording{Message: []byte("abc")},
		&apievents.DesktopRecording{Message: []byte("def")},
		&apievents.DesktopRecording{Message: []byte("ghi")},
		&apievents.DesktopRecording{Message: []byte("jkl")},
	}
	s := newServer(t, 20*time.Millisecond, events)

	// connect to the server and verify that we receive
	// all 4 JSON-encoded events
	url := strings.Replace(s.URL, "http", "ws", 1)

	// As per https://pkg.go.dev/github.com/gorilla/websocket#Dialer.DialContext:
	// "The response body may not contain the entire response and does not need to be closed by the application."
	//nolint:bodyclose // false positive
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)

	t.Cleanup(func() { ws.Close() })

	for _, evt := range events {
		typ, b, err := ws.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.BinaryMessage, typ)

		var dr apievents.DesktopRecording
		err = utils.FastUnmarshal(b, &dr)
		require.NoError(t, err)
		require.Equal(t, evt.(*apievents.DesktopRecording).Message, dr.Message)
	}

	typ, b, err := ws.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.BinaryMessage, typ)
	require.JSONEq(t, `{"message":"end"}`, string(b))
}

func newServer(t *testing.T, streamInterval time.Duration, events []apievents.AuditEvent) *httptest.Server {
	t.Helper()

	fs := eventstest.NewFakeStreamer(events, streamInterval)
	log := utils.NewLoggerForTests()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()

		player, err := player.New(&player.Config{
			Clock:     clockwork.NewRealClock(),
			Log:       log,
			SessionID: session.ID("session-id"),
			Streamer:  fs,
		})
		assert.NoError(t, err)
		player.Play()
		desktop.PlayRecording(r.Context(), log, ws, player)
	}))

	t.Cleanup(s.Close)
	return s
}
