/**
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
	"context"
	"encoding/binary"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

func TestResizeTerminalEvent(t *testing.T) {
	tests := []struct {
		name         string
		sizeString   string
		expectResize bool
	}{
		{
			name:         "valid size string",
			sizeString:   "80:24",
			expectResize: true,
		},
		{
			name:         "invalid format - no colon",
			sizeString:   "8024",
			expectResize: false,
		},
		{
			name:         "invalid format - too many parts",
			sizeString:   "80:24:10",
			expectResize: false,
		},
		{
			name:         "invalid cols",
			sizeString:   "abc:24",
			expectResize: false,
		},
		{
			name:         "invalid rows",
			sizeString:   "80:xyz",
			expectResize: false,
		},
		{
			name:         "empty string",
			sizeString:   "",
			expectResize: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &recordingPlayback{
				ctx:    t.Context(),
				logger: slog.Default(),
			}
			s.terminal.vt = vt10x.New()

			initialCols, initialRows := s.terminal.vt.Size()

			s.resizeTerminal(tt.sizeString)

			if tt.expectResize {
				parts := strings.Split(tt.sizeString, ":")
				expectedCols, _ := strconv.Atoi(parts[0])
				expectedRows, _ := strconv.Atoi(parts[1])

				newCols, newRows := s.terminal.vt.Size()
				require.Equal(t, expectedCols, newCols)
				require.Equal(t, expectedRows, newRows)
			} else {
				newCols, newRows := s.terminal.vt.Size()

				require.Equal(t, initialCols, newCols)
				require.Equal(t, initialRows, newRows)
			}
		})
	}
}

func TestCreateTaskContext(t *testing.T) {
	s := &recordingPlayback{
		ctx:    t.Context(),
		logger: slog.Default(),
	}

	// Create first task
	taskCtx1 := s.createTaskContext()
	require.NotNil(t, taskCtx1)
	require.NotNil(t, s.cancelActiveTask)

	require.NoError(t, taskCtx1.Err(), "task context should not be canceled yet")

	// Create second task - should cancel the first task
	taskCtx2 := s.createTaskContext()
	require.NotNil(t, taskCtx2)

	// First task should be canceled
	select {
	case <-taskCtx1.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("First task context should be canceled")
	}

	// Second task should still be active
	select {
	case <-taskCtx2.Done():
		t.Fatal("Second task context should not be canceled yet")
	default:
		// Expected
	}
}

func TestFetchOverWebSocket(t *testing.T) {
	ws, _ := createWebSocket(t, func(mockClient *mockStreamClient) {
		<-mockClient.eventRequested

		mockClient.sendEvent(&apievents.SessionStart{
			TerminalSize: "80:24",
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 500,
			Data:              []byte("Hello, World!"),
		})
		mockClient.sendEvent(&apievents.SessionEnd{
			StartTime: time.Now(),
			EndTime:   time.Now().Add(600 * time.Millisecond),
		})
	})

	responses := fetchAndCollectResponses(t, ws, 0, 1000, false, 1)

	require.Len(t, responses, 3, "Should receive 3 messages: start, print, stop")

	require.Equal(t, byte(eventTypeStart), responses[0][0], "First message should be start event")
	require.Equal(t, byte(eventTypeBatch), responses[1][0], "Second message should be batch event")
	require.Equal(t, byte(eventTypeStop), responses[2][0], "Third message should be stop event")

	require.Contains(t, string(responses[1][responseHeaderSize:]), "Hello, World!", "Print event should contain expected data")
}

func TestErrorOverWebSocket(t *testing.T) {
	ws, _ := createWebSocket(t, func(mockClient *mockStreamClient) {
		<-mockClient.eventRequested

		mockClient.sendError(errors.New("test error"))
	})

	responses := fetchAndCollectResponses(t, ws, 0, 1000, false, 1)

	require.Len(t, responses, 3, "Should receive 3 messages: start, error, stop")

	require.Equal(t, byte(eventTypeStart), responses[0][0], "First message should be start event")
	require.Equal(t, byte(eventTypeError), responses[1][0], "Second message should be error event")
	require.Equal(t, byte(eventTypeStop), responses[2][0], "Third message should be stop event")

	require.Contains(t, string(responses[1][responseHeaderSize:]), "test error", "Error event should contain expected error message")
}

func TestSeekingBackwards(t *testing.T) {
	ws, mockClient := createWebSocket(t, func(mockClient *mockStreamClient) {
		// no need to send events, just testing that seeking forwards reuses the same stream and
		// seeking backwards starts a new stream
	})

	req := createFetchRequest(0, 1000, 1, false)

	err := ws.WriteMessage(websocket.BinaryMessage, req)
	require.NoError(t, err)

	select {
	case <-mockClient.eventRequested:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for event request")
	}

	require.Equal(t, 1, mockClient.streamCount, "Should have started a single stream")

	req = createFetchRequest(1000, 2000, 2, false)
	err = ws.WriteMessage(websocket.BinaryMessage, req)
	require.NoError(t, err)

	require.Equal(t, 1, mockClient.streamCount, "Should still be on the same stream after seeking forwards")

	req = createFetchRequest(0, 1000, 3, false)
	err = ws.WriteMessage(websocket.BinaryMessage, req)
	require.NoError(t, err)

	select {
	case <-mockClient.eventRequested:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for event request")
	}

	require.Equal(t, 2, mockClient.streamCount, "Should have started a new stream after seeking backwards")
}

func TestRequestScreen(t *testing.T) {
	ws, _ := createWebSocket(t, func(mockClient *mockStreamClient) {
		<-mockClient.eventRequested

		mockClient.sendEvent(&apievents.SessionStart{
			TerminalSize: "80:24",
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 500,
			Data:              []byte("\x1b[H\x1b[2JHello, World!"),
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 700,
			Data:              []byte("\033[2J\033[H"), // send a clear screen escape sequence
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 1000,
			Data:              []byte("\x1b[H\x1b[2JThis is the second screen update"),
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 1500,
			Data:              []byte("\x1b[H\x1b[2JThis is the third screen update"),
		})
		mockClient.sendEvent(&apievents.SessionEnd{
			StartTime: time.Now(),
			EndTime:   time.Now().Add(2 * time.Second),
		})
	})

	responses := fetchAndCollectResponses(t, ws, 1200, 2200, true, 1)

	require.Len(t, responses, 4, "Should receive 4 messages: start, screen, batch, stop")

	require.Equal(t, byte(eventTypeStart), responses[0][0], "First message should be start event")
	require.Equal(t, byte(eventTypeScreen), responses[1][0], "Second message should be screen event")

	require.Contains(t, string(responses[1][responseHeaderSize:]), "This is the second screen update", "Screen event should contain data from after the clear screen sequence")
	require.NotContains(t, string(responses[1][responseHeaderSize:]), "Hello, World!", "Screen event should not contain data from before the clear screen sequence")

	require.Equal(t, byte(eventTypeBatch), responses[2][0], "Third message should be batch event")
	require.Equal(t, byte(eventTypeStop), responses[3][0], "Fourth message should be stop event")
}

func TestResizeEvent(t *testing.T) {
	ws, _ := createWebSocket(t, func(mockClient *mockStreamClient) {
		<-mockClient.eventRequested

		mockClient.sendEvent(&apievents.SessionStart{
			TerminalSize: "80:24",
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 500,
			Data:              []byte("\x1b[H\x1b[2JHello, World!"),
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 700,
			Data:              []byte("\033[2J\033[H"), // send a clear screen escape sequence
		})
		mockClient.sendEvent(&apievents.Resize{
			TerminalSize: "100:30",
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 1100,
			Data:              []byte("\x1b[H\x1b[2JThis is the second screen update"),
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 1500,
			Data:              []byte("\x1b[H\x1b[2JThis is the third screen update"),
		})
		mockClient.sendEvent(&apievents.SessionEnd{
			StartTime: time.Now(),
			EndTime:   time.Now().Add(2 * time.Second),
		})
	})

	responses := fetchAndCollectResponses(t, ws, 0, 1000, true, 1)

	require.Len(t, responses, 4, "Should receive 4 messages: start, screen, batch, stop")

	require.Equal(t, byte(eventTypeStart), responses[0][0], "First message should be start event")
	require.Equal(t, byte(eventTypeScreen), responses[1][0], "Second message should be screen event")

	require.Contains(t, string(responses[1][responseHeaderSize:]), "[8;24;80t", "Initial screen event should have 24 rows and 80 columns")

	responses = fetchAndCollectResponses(t, ws, 1000, 2000, true, 2)

	require.Contains(t, string(responses[1][responseHeaderSize:]), "[8;30;100", "Initial screen event should have 30 rows and 100 columns")
}

func TestBufferedEvents(t *testing.T) {
	ws, _ := createWebSocket(t, func(mockClient *mockStreamClient) {
		<-mockClient.eventRequested

		mockClient.sendEvent(&apievents.SessionStart{
			TerminalSize: "80:24",
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 500,
			Data:              []byte("Will be included in the first batch"),
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 999,
			Data:              []byte("Will only just make it into the first batch"),
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 1001,
			Data:              []byte("Will be included in the second batch"),
		})
		mockClient.sendEvent(&apievents.SessionEnd{
			StartTime: time.Now(),
			EndTime:   time.Now().Add(2 * time.Second),
		})
	})

	responses := fetchAndCollectResponses(t, ws, 0, 1000, false, 1)

	require.Len(t, responses, 3, "Should receive 3 messages: start, batch, stop")

	require.Equal(t, byte(eventTypeStart), responses[0][0], "First message should be start event")
	require.Equal(t, byte(eventTypeBatch), responses[1][0], "Second message should be batch event")

	require.Contains(t, string(responses[1][responseHeaderSize:]), "Will only just make it into the first batch", "First batch should contain expected data")
	require.NotContains(t, string(responses[1][responseHeaderSize:]), "Will be included in the second batch", "First batch should not contain data from the third print event")

	require.Equal(t, byte(eventTypeStop), responses[2][0], "Third message should be stop event")

	responses = fetchAndCollectResponses(t, ws, 1000, 2000, false, 2)

	require.Len(t, responses, 3, "Should receive 3 messages: start, batch, stop")

	require.Equal(t, byte(eventTypeStart), responses[0][0], "First message should be start event")
	require.Equal(t, byte(eventTypeBatch), responses[1][0], "Second message should be batch event")

	require.Contains(t, string(responses[1][responseHeaderSize:]), "Will be included in the second batch", "Second batch should contain expected data")

	require.Equal(t, byte(eventTypeStop), responses[2][0], "Third message should be stop event")
}

func TestBufferedEvents_LargeGap(t *testing.T) {
	ws, _ := createWebSocket(t, func(mockClient *mockStreamClient) {
		<-mockClient.eventRequested

		mockClient.sendEvent(&apievents.SessionStart{
			TerminalSize: "80:24",
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 500,
			Data:              []byte("a"),
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 999,
			Data:              []byte("b"),
		})

		// we will request the first second and then the 9th-10th seconds
		// adding some events in the middle to ensure the events are processed properly and the terminal state is correct
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 2000,
			Data:              []byte("c"),
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 3000,
			Data:              []byte("d"),
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 4000,
			Data:              []byte("e"),
		})

		// add some events into the time range that will be requested
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 9000,
			Data:              []byte("f"),
		})
		mockClient.sendEvent(&apievents.SessionPrint{
			DelayMilliseconds: 9500,
			Data:              []byte("g"),
		})

		mockClient.sendEvent(&apievents.SessionEnd{
			StartTime: time.Now(),
			EndTime:   time.Now().Add(10 * time.Second),
		})
	})

	responses := fetchAndCollectResponses(t, ws, 0, 1000, false, 1)

	require.Len(t, responses, 3, "Should receive 3 messages: start, batch, stop")

	require.Equal(t, byte(eventTypeStart), responses[0][0], "First message should be start event")
	require.Equal(t, byte(eventTypeBatch), responses[1][0], "Second message should be batch event")

	require.Contains(t, string(responses[1][responseHeaderSize:]), "a", "First batch should contain event with 'a' data")
	require.Contains(t, string(responses[1][responseHeaderSize:]), "b", "First batch should contain event with 'b' data")

	require.Equal(t, byte(eventTypeStop), responses[2][0], "Third message should be stop event")

	responses = fetchAndCollectResponses(t, ws, 9000, 10000, true, 2)

	require.Len(t, responses, 4, "Should receive 4 messages: start, screen, batch, stop")

	require.Equal(t, byte(eventTypeStart), responses[0][0], "First message should be start event")
	require.Equal(t, byte(eventTypeScreen), responses[1][0], "Second message should be screen event")

	require.Contains(t, string(responses[1][responseHeaderSize:]), "abcde", "Screen event should contain data from all previous events")
	require.NotContains(t, string(responses[1][responseHeaderSize:]), "f", "Screen event should not contain data from the future events")

	require.Equal(t, byte(eventTypeBatch), responses[2][0], "Third message should be batch event")

	require.Contains(t, string(responses[2][responseHeaderSize:]), "f", "Second batch should contain event with f data")
	require.Contains(t, string(responses[2][responseHeaderSize:]), "g", "Second batch should contain event with g data")

	require.Equal(t, byte(eventTypeStop), responses[3][0], "Fourth message should be stop event")
}

func TestUnsupportedRequest(t *testing.T) {
	mockClient := newMockStreamClient()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		ws, _ := upgrader.Upgrade(w, r, nil)
		defer ws.Close()

		ctx := context.Background()
		logger := slog.Default()

		playback := newRecordingPlayback(ctx, ws, mockClient, "test-session", logger)
		playback.run()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	if resp != nil {
		resp.Body.Close()
	}
	defer ws.Close()

	// Send a request with an invalid request type
	req := make([]byte, requestHeaderSize)
	req[0] = 99 // invalid request type

	err = ws.WriteMessage(websocket.BinaryMessage, req)
	require.NoError(t, err)

	ws.SetReadDeadline(time.Now().Add(5 * time.Second))

	_, data, err := ws.ReadMessage()

	// Should receive a message with the type set to eventTypeError
	require.NoError(t, err)
	require.Equal(t, byte(eventTypeError), data[0], "Response should be an error event")
	require.Contains(t, string(data[responseHeaderSize:]), "unknown request type", "Error message should indicate unsupported request type")

	deadline := time.Now().Add(time.Second)
	ws.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		deadline)
}

func TestNonBinaryMessageClosesWebSocket(t *testing.T) {
	mockClient := newMockStreamClient()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		ws, _ := upgrader.Upgrade(w, r, nil)
		defer ws.Close()

		ctx := context.Background()
		logger := slog.Default()

		playback := newRecordingPlayback(ctx, ws, mockClient, "test-session", logger)
		playback.run()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	if resp != nil {
		resp.Body.Close()
	}
	defer ws.Close()

	// Send a text message (non-binary) - this should trigger the close handshake
	err = ws.WriteMessage(websocket.TextMessage, []byte("this is a text message"))
	require.NoError(t, err)

	ws.SetReadDeadline(time.Now().Add(5 * time.Second))

	// The server should send a close message with CloseUnsupportedData
	_, _, err = ws.ReadMessage()
	require.Error(t, err)

	var closeErr *websocket.CloseError

	require.ErrorAs(t, err, &closeErr, "Expected a websocket.CloseError, got %T: %v", err, err)
	require.Equal(t, websocket.CloseUnsupportedData, closeErr.Code, "Expected CloseUnsupportedData (1003), got %d", closeErr.Code)
	require.Contains(t, closeErr.Text, "only binary messages are supported", "Close message should explain that only binary messages are supported")

	err = ws.WriteMessage(websocket.BinaryMessage, createFetchRequest(0, 1000, 1, false))

	require.Error(t, err, "Should not be able to send messages after close handshake")
}

func createFetchRequest(start, end int64, requestID uint32, currentScreen bool) []byte {
	buf := make([]byte, requestHeaderSize)

	buf[0] = byte(requestTypeFetch)

	binary.BigEndian.PutUint64(buf[1:9], uint64(start))
	binary.BigEndian.PutUint64(buf[9:17], uint64(end))
	binary.BigEndian.PutUint32(buf[17:21], requestID)

	buf[21] = 0
	if currentScreen {
		buf[21] = 1
	}

	return buf
}

// fetchAndCollectResponses sends a fetch request over the WebSocket connection and collects all responses.
func fetchAndCollectResponses(t *testing.T, ws *websocket.Conn, start, end int64, requestCurrentScreen bool, requestID uint32) [][]byte {
	testTimeout := 5 * time.Second

	req := createFetchRequest(start, end, requestID, requestCurrentScreen)

	err := ws.WriteMessage(websocket.BinaryMessage, req)
	require.NoError(t, err)

	var responses [][]byte

	done := make(chan bool)
	go func() {
		for {
			ws.SetReadDeadline(time.Now().Add(testTimeout))
			_, msg, err := ws.ReadMessage()
			if err != nil {
				break
			}

			responses = append(responses, msg)
			if len(msg) > 0 && msg[0] == byte(eventTypeStop) {
				done <- true
				break
			}
		}
	}()

	select {
	case success := <-done:
		if !success {
			t.Fatal("Connection closed before receiving stop event")
		}
	case <-time.After(testTimeout):
		t.Fatal("Timeout waiting for responses")
	}

	return responses
}

// createWebSocket sets up a WebSocket server for testing, returning the server, websocket connection and mock
// client, taking a callback to allow populating the mock client with events before running the playback.
func createWebSocket(t *testing.T, setupEvents func(mockClient *mockStreamClient)) (*websocket.Conn, *mockStreamClient) {
	mockClient := newMockStreamClient()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		ws, _ := upgrader.Upgrade(w, r, nil)
		defer ws.Close()

		ctx := context.Background()
		logger := slog.Default()

		playback := newRecordingPlayback(ctx, ws, mockClient, "test-session", logger)

		go setupEvents(mockClient)

		playback.run()
	}))

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	if resp != nil {
		resp.Body.Close()
	}

	t.Cleanup(func() {
		deadline := time.Now().Add(time.Second)
		ws.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			deadline)

		ws.Close()
		server.Close()
	})

	return ws, mockClient
}

// mockStreamClient implements events.SessionStreamer interface for testing, counting the number of streams
// and allowing sending events and errors to the channel.
type mockStreamClient struct {
	events         chan apievents.AuditEvent
	errors         chan error
	eventRequested chan struct{}
	streamCount    int
}

func newMockStreamClient() *mockStreamClient {
	return &mockStreamClient{
		events:         make(chan apievents.AuditEvent, 100),
		errors:         make(chan error, 1),
		eventRequested: make(chan struct{}, 10),
		streamCount:    0,
	}
}

func (m *mockStreamClient) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	m.streamCount += 1

	// Notify that a new stream has been requested so that the test can send events to the channels.
	select {
	case m.eventRequested <- struct{}{}:
	default:
	}

	return m.events, m.errors
}

func (m *mockStreamClient) sendEvent(evt apievents.AuditEvent) {
	m.events <- evt
}

func (m *mockStreamClient) sendError(err error) {
	m.errors <- err
}
