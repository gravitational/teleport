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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
	"github.com/gravitational/teleport/lib/utils/log/logtest"
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
	log := logtest.NewLogger()

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
		desktop.StreamRecording(r.Context(), log, ws, player)
	}))

	t.Cleanup(s.Close)
	return s
}

type mockJSONReader struct {
	json chan json.RawMessage
	err  chan error
	once sync.Once
}

func (m *mockJSONReader) Close() {
	m.once.Do(func() {
		close(m.json)
	})
}

func (m *mockJSONReader) ReadJSON(v interface{}) error {
	select {
	case jsn, ok := <-m.json:
		if !ok {
			return io.EOF
		}
		return json.Unmarshal(jsn, v)
	case e := <-m.err:
		return e
	}
}

type mockPlayer struct {
	setPosCount   int
	setSpeedCount int
	speedSettings []float64
	pauseCount    int
	playCount     int
}

func (m *mockPlayer) SetPos(d time.Duration) error {
	m.setPosCount++
	return nil
}

func (m *mockPlayer) SetSpeed(s float64) error {
	m.setSpeedCount++
	m.speedSettings = append(m.speedSettings, s)
	return nil
}

func (m *mockPlayer) Pause() error {
	m.pauseCount++
	return nil
}

func (m *mockPlayer) Play() error {
	m.playCount++
	return nil
}

func jsonActionMessage(action string, speed float64, pos int64) json.RawMessage {
	if speed > 0 {
		return fmt.Appendf(nil, `{"action": "%s", "speed": %f, "pos": %d}`, action, speed, pos)
	}
	return fmt.Appendf(nil, `{"action": "%s", "pos": %d}`, action, pos)

}

func sendWithDeadline[T any](ctx context.Context, c chan T, val T) {
	select {
	case c <- val:
	case <-ctx.Done():
	}
}

func TestPlaybackActions(t *testing.T) {
	newFixture := func() (*mockJSONReader, *mockPlayer, chan error) {

		mockReader := &mockJSONReader{
			json: make(chan json.RawMessage),
			err:  make(chan error),
		}

		mockPlayer := &mockPlayer{}

		errCh := make(chan error)
		go func() {
			errCh <- desktop.ReceivePlaybackActions(t.Context(), slog.New(slog.DiscardHandler), mockReader, mockPlayer)
		}()

		return mockReader, mockPlayer, errCh
	}

	t.Run("invalid-action", func(t *testing.T) {
		mockReader, mockPlayer, errCh := newFixture()
		defer mockReader.Close()

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		sendWithDeadline(ctx, mockReader.json, jsonActionMessage("invalid", 0, 0))
		select {
		case err := <-errCh:
			require.Error(t, err)
		case <-ctx.Done():
			t.Fatalf("playback action reader did not exit before deadline")
		}
		assert.Zero(t, mockPlayer.playCount)
		assert.Zero(t, mockPlayer.pauseCount)
		assert.Zero(t, mockPlayer.setPosCount)
		assert.Zero(t, mockPlayer.setSpeedCount)
	})

	t.Run("invalid-speeds", func(t *testing.T) {
		mockReader, mockPlayer, errCh := newFixture()
		defer mockReader.Close()

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		// Too low
		sendWithDeadline(ctx, mockReader.json, jsonActionMessage("speed", 0.01, 0))
		// Too high
		sendWithDeadline(ctx, mockReader.json, jsonActionMessage("speed", 17, 0))
		mockReader.Close()
		select {
		case err := <-errCh:
			require.Error(t, err)
		case <-ctx.Done():
			t.Fatalf("playback action reader did not exit before deadline")
		}
		assert.Zero(t, mockPlayer.playCount)
		assert.Zero(t, mockPlayer.pauseCount)
		assert.Zero(t, mockPlayer.setPosCount)
		assert.Equal(t, 2, mockPlayer.setSpeedCount)
		assert.Contains(t, mockPlayer.speedSettings, 0.25)
		assert.Contains(t, mockPlayer.speedSettings, float64(16))
	})

	t.Run("happy-paths", func(t *testing.T) {
		mockReader, mockPlayer, errCh := newFixture()
		defer mockReader.Close()

		ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
		defer cancel()

		for _, action := range []json.RawMessage{
			jsonActionMessage("speed", 0.25, 0),
			jsonActionMessage("speed", 16, 0),
			jsonActionMessage("play/pause", 16, 0),
			jsonActionMessage("play/pause", 16, 0),
			jsonActionMessage("seek", 0, 1000),
		} {
			sendWithDeadline(ctx, mockReader.json, action)
		}
		// Should return EOF on error
		mockReader.Close()

		select {
		case err := <-errCh:
			require.ErrorIs(t, err, io.EOF, "expected ReceivePlaybackActions to return an EOF but got %v", err)
		case <-ctx.Done():
			t.Fatalf("playback action reader did not exit before deadline")
		}
		assert.Equal(t, 1, mockPlayer.playCount)
		assert.Equal(t, 1, mockPlayer.pauseCount)
		assert.Equal(t, 1, mockPlayer.setPosCount)
		assert.Equal(t, 2, mockPlayer.setSpeedCount)
	})
}
