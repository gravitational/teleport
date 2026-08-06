/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

var interactionPayload = `{
	"type":"interactive",
	"envelope_id":"env-1",
	"payload":{
		"type":"block_actions",
		"user":{"id":"U1"},
		"channel":{"id":"C1"},
		"actions":[{"type":"button","action_id":"approve_button","value":"req-1"}]
	}
}`

var malformedPayload = `{ malformed }`

var nonBlockActionsPayload = `{
	"type":"interactive",
	"envelope_id":"env-1",
	"payload":{
		"type":"slash"
	}
}`

var disconnectPayload = `{
  	"type": "disconnect",
  	"reason": "warning",
  	"debug_info": {
    	"host": "example_slack_host"
  	}
}`

func newTestSocket(t *testing.T, onConn func(*websocket.Conn)) *websocket.Conn {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		serverConn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer serverConn.Close()

		onConn(serverConn)
	}))

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	clientConn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	require.NoError(t, err)

	t.Cleanup(func() {
		srv.Close()
	})

	return clientConn
}

func TestReadPump_WebSocketCloseError(t *testing.T) {
	clientConn := newTestSocket(t, func(ws *websocket.Conn) {
		require.NoError(t, ws.WriteMessage(websocket.TextMessage, []byte(disconnectPayload)))
		require.NoError(t, ws.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Time{}))
	})
	defer clientConn.Close()

	client := &SocketModeClient{
		interactionsCh: make(chan InteractionEvent, 1),
	}

	rawCh := make(chan json.RawMessage, 1)

	err := client.readPump(t.Context(), clientConn, rawCh)

	select {
	case raw := <-rawCh:
		require.JSONEq(t, disconnectPayload, string(raw))
	default:
		t.Fatal("failed to receive disconnect payload")
	}
	// WebSocket close should return an error in order to trigger reconnect.
	require.Error(t, err)
}

func TestParseEvents(t *testing.T) {
	tests := []struct {
		name           string
		payload        string
		hasInteraction bool
		wantAck        bool
		wantErr        error
	}{
		{
			name:           "interaction payload",
			payload:        interactionPayload,
			wantAck:        true,
			hasInteraction: true,
		},
		{
			name:    "malformed payload; skip",
			payload: malformedPayload,
		},
		{
			name:    "non block_actions payload; ack & skip",
			payload: nonBlockActionsPayload,
			wantAck: true,
		},
		{
			name:    "disconnect",
			payload: disconnectPayload,
			wantErr: ErrSocketModeDisconnect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				client := &SocketModeClient{
					interactionsCh: make(chan InteractionEvent, 1),
				}

				rawCh := make(chan json.RawMessage, 1)
				ackCh := make(chan string, 1)

				done := make(chan error)
				go func() {
					done <- client.parseEvents(ctx, rawCh, ackCh)
				}()

				rawCh <- json.RawMessage(tt.payload)

				synctest.Wait()
				if tt.wantAck {
					select {
					case envelopeID := <-ackCh:
						require.Equal(t, "env-1", envelopeID)
					default:
						t.Fatal("failed to receive ack")
					}
				}

				require.Equal(t, tt.hasInteraction, len(client.Interactions()) > 0)

				cancel()
				synctest.Wait()
				if tt.wantErr != nil {
					require.ErrorIs(t, <-done, tt.wantErr)
				} else {
					require.ErrorIs(t, <-done, context.Canceled)
				}
			})
		})
	}
}
