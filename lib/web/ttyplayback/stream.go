/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package ttyplayback

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/metadata"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/ttyplayback/terminal"
	"github.com/gravitational/teleport/lib/web/ttyplayback/vt10x"
)

const (
	// RequestTypeFetch requests event data
	RequestTypeFetch byte = 1
	// RequestTypeClose closes the connection
	RequestTypeClose byte = 2

	// EventTypeStart indicates stream start
	EventTypeStart byte = 1
	// EventTypeStop indicates stream stop
	EventTypeStop byte = 2
	// EventTypeError indicates an error
	EventTypeError byte = 3
	// EventTypeSessionStart indicates session started
	EventTypeSessionStart byte = 4
	// EventTypeSessionPrint contains terminal output
	EventTypeSessionPrint byte = 5
	// EventTypeSessionEnd indicates session ended
	EventTypeSessionEnd byte = 6
	// EventTypeResize indicates terminal resize
	EventTypeResize byte = 7
	// EventTypeScreen contains terminal screen state
	EventTypeScreen byte = 8
	// EventTypeBatch indicates a batch of events
	EventTypeBatch byte = 9

	requestHeaderSize  = 21
	responseHeaderSize = 21
)

// BinaryRequest represents a client request
type BinaryRequest struct {
	Type                 byte
	StartTime            int64
	EndTime              int64
	RequestID            int
	RequestCurrentScreen bool
}

// SessionEventsHandler manages session event streaming
type SessionEventsHandler struct {
	ctx       context.Context
	cancel    context.CancelFunc
	ws        *websocket.Conn
	clt       authclient.ClientI
	sessionID string
	logger    *slog.Logger

	mu         sync.Mutex
	activeTask context.CancelFunc

	// wsMu protects websocket writes
	wsMu sync.Mutex
}

// NewSessionEventsHandler creates a new session events handler
func NewSessionEventsHandler(ctx context.Context, ws *websocket.Conn, clt authclient.ClientI, sessionID string, logger *slog.Logger) *SessionEventsHandler {
	ctx, cancel := context.WithCancel(ctx)
	return &SessionEventsHandler{
		ctx:       ctx,
		cancel:    cancel,
		ws:        ws,
		clt:       clt,
		sessionID: sessionID,
		logger:    logger,
	}
}

// Run starts the handler
func (s *SessionEventsHandler) Run() {
	defer s.cleanup()
	s.readLoop()
}

func (s *SessionEventsHandler) cleanup() {
	s.cancel()

	s.wsMu.Lock()
	_ = s.ws.WriteMessage(websocket.CloseMessage, nil)
	_ = s.ws.Close()
	s.wsMu.Unlock()

	s.mu.Lock()
	if s.activeTask != nil {
		s.activeTask()
	}
	s.mu.Unlock()
}

func (s *SessionEventsHandler) readLoop() {
	for {
		msgType, data, err := s.ws.ReadMessage()
		if err != nil {
			if !utils.IsOKNetworkError(err) {
				s.logger.WarnContext(s.ctx, "websocket read error", "error", err)
			}
			return
		}

		if msgType != websocket.BinaryMessage {
			continue
		}

		req, err := decodeBinaryRequest(data)
		if err != nil {
			s.logger.WarnContext(s.ctx, "failed to decode request", "error", err)
			continue
		}

		switch req.Type {
		case RequestTypeFetch:
			s.handleFetchRequest(req)
		case RequestTypeClose:
			return
		}
	}
}

func encodeScreenEvent(vt vt10x.Terminal) []byte {
	screen := terminal.Serialize(vt)
	data := vt.ANSI()

	eventData := make([]byte, 21+len(data))
	eventData[0] = EventTypeScreen

	binary.BigEndian.PutUint32(eventData[1:5], uint32(screen.Cols))
	binary.BigEndian.PutUint32(eventData[5:9], uint32(screen.Rows))
	binary.BigEndian.PutUint32(eventData[9:13], uint32(screen.CursorX))
	binary.BigEndian.PutUint32(eventData[13:17], uint32(screen.CursorY))
	binary.BigEndian.PutUint32(eventData[17:21], uint32(len(data)))

	copy(eventData[21:], data)

	return eventData
}

func (s *SessionEventsHandler) handleFetchRequest(req *BinaryRequest) {
	s.mu.Lock()
	if s.activeTask != nil {
		s.activeTask()
	}
	taskCtx, taskCancel := context.WithCancel(s.ctx)
	s.activeTask = taskCancel
	s.mu.Unlock()

	go s.streamEvents(taskCtx, req)
}

func (s *SessionEventsHandler) streamEvents(ctx context.Context, req *BinaryRequest) {
	defer func() {
		s.mu.Lock()
		s.activeTask = nil
		s.mu.Unlock()
	}()

	s.sendEvent(EventTypeStart, req.StartTime, nil, 0, req.RequestID)

	fmt.Println("Streaming session events for session ID:", s.sessionID)

	events, errors := s.clt.StreamSessionEvents(
		metadata.WithSessionRecordingFormatContext(ctx, teleport.PTY),
		session.ID(s.sessionID),
		0,
	)

	vt := vt10x.New()
	currentTime := int64(0)
	index := 0
	screenSent := false
	inTimeRange := req.StartTime == 0 // If starting from beginning, we're already in range
	var streamStartTime time.Time

	const maxBatchSize = 200
	eventBatch := make([]struct {
		eventType byte
		timestamp int64
		data      []byte
		index     int
	}, 0, maxBatchSize)

	flushBatch := func() {
		if len(eventBatch) == 0 {
			return
		}
		s.sendEventBatch(eventBatch, req.RequestID)
		eventBatch = eventBatch[:0]
	}

	addToBatch := func(eventType byte, timestamp int64, data []byte, idx int) {
		eventBatch = append(eventBatch, struct {
			eventType byte
			timestamp int64
			data      []byte
			index     int
		}{eventType, timestamp, data, idx})

		if len(eventBatch) >= maxBatchSize {
			flushBatch()
		}
	}

	for {
		select {
		case <-ctx.Done():
			flushBatch()
			s.sendEvent(EventTypeStop, 0, encodeTime(req.StartTime, req.EndTime), 0, req.RequestID)
			return
		case err := <-errors:
			flushBatch()
			if err != nil {
				s.sendError(err, req.RequestID)
			}
			s.sendEvent(EventTypeStop, 0, encodeTime(req.StartTime, req.EndTime), 0, req.RequestID)
			return
		case evt, ok := <-events:
			if !ok {
				flushBatch()
				if req.RequestCurrentScreen && !screenSent && inTimeRange {
					s.sendScreenState(vt, currentTime, req.RequestID)
				}
				s.sendEvent(EventTypeStop, 0, encodeTime(req.StartTime, req.EndTime), 0, req.RequestID)
				return
			}

			if _, ok := evt.(*apievents.SessionStart); ok && streamStartTime.IsZero() {
				streamStartTime = evt.GetTime()
			}

			eventTime := getEventTime(evt)

			// Process all events with vt10x to maintain terminal state
			switch evt := evt.(type) {
			case *apievents.SessionStart:
				resizeTerminal(vt, evt.TerminalSize)
				if inTimeRange {
					addToBatch(EventTypeSessionStart, 0, []byte(evt.TerminalSize), index)
					index++
				}

			case *apievents.SessionPrint:
				// Always write to vt to maintain terminal state
				_, _ = vt.Write(evt.Data)
				currentTime = evt.DelayMilliseconds

				// Check if we've entered the time range
				if !inTimeRange && evt.DelayMilliseconds >= req.StartTime {
					inTimeRange = true
					if req.RequestCurrentScreen && !screenSent {
						flushBatch()
						s.sendScreenState(vt, evt.DelayMilliseconds, req.RequestID)
						screenSent = true
					}
				}

				// Only send events within the time range
				if inTimeRange && evt.DelayMilliseconds <= req.EndTime {
					addToBatch(EventTypeSessionPrint, evt.DelayMilliseconds, evt.Data, index)
					index++
				} else if evt.DelayMilliseconds > req.EndTime {
					flushBatch()
					s.sendEvent(EventTypeStop, 0, encodeTime(req.StartTime, req.EndTime), 0, req.RequestID)
					return
				}

			case *apievents.SessionEnd:
				endTime := int64(evt.EndTime.Sub(evt.StartTime) / time.Millisecond)
				if endTime >= req.StartTime && endTime <= req.EndTime {
					addToBatch(EventTypeSessionEnd, endTime, []byte(evt.EndTime.Format(time.RFC3339)), index)
				}
				flushBatch()
				s.sendEvent(EventTypeStop, 0, encodeTime(req.StartTime, req.EndTime), 0, req.RequestID)
				return

			case *apievents.Resize:
				// Always resize vt to maintain terminal state
				resizeTerminal(vt, evt.TerminalSize)

				if inTimeRange {
					addToBatch(EventTypeResize, 0, []byte(evt.TerminalSize), index)
					index++
				}
			}
		}
	}
}

func (s *SessionEventsHandler) sendEvent(eventType byte, timestamp int64, data []byte, index int, requestID int) {
	buf := make([]byte, responseHeaderSize+len(data))
	buf[0] = eventType
	binary.BigEndian.PutUint64(buf[1:9], uint64(timestamp))
	binary.BigEndian.PutUint32(buf[9:13], uint32(len(data)))
	binary.BigEndian.PutUint32(buf[13:17], uint32(index))
	binary.BigEndian.PutUint32(buf[17:21], uint32(requestID))
	copy(buf[21:], data)

	s.wsMu.Lock()
	err := s.ws.WriteMessage(websocket.BinaryMessage, buf)
	s.wsMu.Unlock()

	if err != nil {
		s.logger.WarnContext(s.ctx, "failed to send event", "error", err)
	}
}

func (s *SessionEventsHandler) sendEventBatch(batch []struct {
	eventType byte
	timestamp int64
	data      []byte
	index     int
}, requestID int) {
	eventsSize := 0
	for _, evt := range batch {
		eventsSize += responseHeaderSize + len(evt.data)
	}

	buf := make([]byte, responseHeaderSize+eventsSize)

	buf[0] = EventTypeBatch
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(batch)))
	binary.BigEndian.PutUint32(buf[5:9], uint32(requestID))

	// keep the header size the same for all events
	offset := responseHeaderSize

	for _, evt := range batch {
		buf[offset] = evt.eventType

		binary.BigEndian.PutUint64(buf[offset+1:offset+9], uint64(evt.timestamp))
		binary.BigEndian.PutUint32(buf[offset+9:offset+13], uint32(len(evt.data)))
		binary.BigEndian.PutUint32(buf[offset+13:offset+17], uint32(evt.index))
		binary.BigEndian.PutUint32(buf[offset+17:offset+21], uint32(requestID))

		copy(buf[offset+21:], evt.data)

		offset += responseHeaderSize + len(evt.data)
	}

	s.wsMu.Lock()
	err := s.ws.WriteMessage(websocket.BinaryMessage, buf)
	s.wsMu.Unlock()

	if err != nil {
		s.logger.WarnContext(s.ctx, "failed to send event batch", "error", err, "batch_size", len(batch))
	}
}

func (s *SessionEventsHandler) sendError(err error, requestID int) {
	s.sendEvent(EventTypeError, 0, []byte(err.Error()), 0, requestID)
}

func (s *SessionEventsHandler) sendScreenState(vt vt10x.Terminal, currentTime int64, requestID int) {
	data := encodeScreenEvent(vt)
	s.sendEvent(EventTypeScreen, currentTime, data, 0, requestID)
}

func encodeTime(startTime, endTime int64) []byte {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf, uint64(startTime))
	binary.BigEndian.PutUint64(buf[8:], uint64(endTime))
	return buf
}

func decodeBinaryRequest(data []byte) (*BinaryRequest, error) {
	if len(data) < requestHeaderSize {
		return nil, fmt.Errorf("request too short")
	}

	req := &BinaryRequest{
		Type:      data[0],
		StartTime: int64(binary.BigEndian.Uint64(data[1:9])),
		EndTime:   int64(binary.BigEndian.Uint64(data[9:17])),
		RequestID: int(binary.BigEndian.Uint32(data[17:21])),
	}

	if len(data) > requestHeaderSize {
		req.RequestCurrentScreen = data[21] == 1
	}

	return req, nil
}

func getEventTime(evt apievents.AuditEvent) int64 {
	switch evt := evt.(type) {
	case *apievents.SessionPrint:
		return evt.DelayMilliseconds
	case *apievents.SessionEnd:
		return int64(evt.EndTime.Sub(evt.StartTime) / time.Millisecond)
	default:
		return 0
	}
}

func resizeTerminal(vt vt10x.Terminal, size string) {
	parts := strings.Split(size, ":")
	if len(parts) != 2 {
		return
	}

	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return
	}

	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return
	}

	vt.Resize(width, height)
}
