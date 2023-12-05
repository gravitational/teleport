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

package desktop_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/websocket"

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
	url := strings.Replace(s.URL, "http", "ws", 1)
	cfg, err := websocket.NewConfig(url, "http://localhost")
	require.NoError(t, err)

	// connect to the server and verify that we receive
	// all 4 JSON-encoded events
	ws, err := websocket.DialConfig(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })

	for _, evt := range events {
		b := make([]byte, 4096)
		n, err := ws.Read(b)
		require.NoError(t, err)

		var dr apievents.DesktopRecording
		err = utils.FastUnmarshal(b[:n], &dr)
		require.NoError(t, err)
		require.Equal(t, evt.(*apievents.DesktopRecording).Message, dr.Message)
	}

	b := make([]byte, 4096)
	n, err := ws.Read(b)
	require.NoError(t, err)
	require.JSONEq(t, `{"message":"end"}`, string(b[:n]))
}

func newServer(t *testing.T, streamInterval time.Duration, events []apievents.AuditEvent) *httptest.Server {
	t.Helper()

	fs := eventstest.NewFakeStreamer(events, streamInterval)
	log := utils.NewLoggerForTests()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		websocket.Handler(func(ws *websocket.Conn) {
			player, err := player.New(&player.Config{
				Clock:     clockwork.NewRealClock(),
				Log:       log,
				SessionID: session.ID("session-id"),
				Streamer:  fs,
			})
			assert.NoError(t, err)
			player.Play()
			desktop.PlayRecording(r.Context(), log, ws, player)
		}).ServeHTTP(w, r)
	}))
	t.Cleanup(s.Close)

	return s
}
