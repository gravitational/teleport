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
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/websocket"

	apievents "github.com/gravitational/teleport/api/types/events"
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

	fs := fakeStreamer{
		interval: streamInterval,
		events:   events,
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		websocket.Handler(func(ws *websocket.Conn) {
			desktop.NewPlayer("session-id", ws, fs, utils.NewLoggerForTests()).Play(r.Context())
		}).ServeHTTP(w, r)
	}))
	t.Cleanup(s.Close)

	return s
}

// fakeStreamer streams the provided events, sending one every interval.
// An interval of 0 sends the events immediately, throttled only by the
// ability of the receiver to keep up.
type fakeStreamer struct {
	events   []apievents.AuditEvent
	interval time.Duration
}

func (f fakeStreamer) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	errors := make(chan error, 1)
	events := make(chan apievents.AuditEvent)

	go func() {
		defer close(events)

		for _, event := range f.events {
			if f.interval != 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(f.interval):
				}
			}

			select {
			case <-ctx.Done():
				return
			case events <- event:
			}
		}
	}()

	return events, errors
}
