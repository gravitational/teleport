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
	"cmp"
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/vt10x"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/metadata"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
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
)

const (
	maxRequestRange    = 10 * time.Minute
	requestHeaderSize  = 21
	responseHeaderSize = 17
)

// fetchRequest represents a client request
type fetchRequest struct {
	Type                 byte
	StartTime            int64
	EndTime              int64
	RequestID            int
	RequestCurrentScreen bool
}

// SessionEventsHandler manages session event streaming
type SessionEventsHandler struct {
	ctx        context.Context
	cancel     context.CancelFunc
	clt        authclient.ClientI
	sessionID  string
	logger     *slog.Logger
	mu         sync.Mutex
	activeTask context.CancelFunc

	stream struct {
		sync.Mutex
		eventsChan  <-chan apievents.AuditEvent
		errorsChan  <-chan error
		lastEndTime int64
	}

	websocket struct {
		sync.Mutex
		*websocket.Conn
	}

	terminal struct {
		sync.RWMutex
		vt          vt10x.Terminal
		currentTime int64
	}
}

type sessionEvent struct {
	eventType byte
	timestamp int64
	data      []byte
}

// NewSessionEventsHandler creates a new session events handler
func NewSessionEventsHandler(ctx context.Context, ws *websocket.Conn, clt authclient.ClientI, sessionID string, logger *slog.Logger) *SessionEventsHandler {
	ctx, cancel := context.WithCancel(ctx)

	s := &SessionEventsHandler{
		ctx:       ctx,
		cancel:    cancel,
		clt:       clt,
		sessionID: sessionID,
		logger:    logger,
	}

	s.websocket.Conn = ws

	return s
}

// Run starts the handler
func (s *SessionEventsHandler) Run() {
	defer s.cleanup()
	s.readLoop()
}

func (s *SessionEventsHandler) cleanup() {
	s.cancel()

	s.websocket.Lock()
	_ = s.WriteMessage(websocket.CloseMessage, nil)
	_ = s.websocket.Close()
	s.websocket.Unlock()
}

func (s *SessionEventsHandler) readLoop() {
	for {
		msgType, data, err := s.websocket.ReadMessage()
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

func (s *SessionEventsHandler) createTaskContext() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeTask != nil {
		s.activeTask()
	}

	ctx, taskCancel := context.WithCancel(s.ctx)
	s.activeTask = taskCancel

	return ctx
}

func (s *SessionEventsHandler) handleFetchRequest(req *fetchRequest) {
	if err := validateRequest(req); err != nil {
		s.sendError(err, req.RequestID)

		return
	}

	ctx := s.createTaskContext()

	s.stream.Lock()
	needNewStream := false

	if s.stream.eventsChan == nil {
		needNewStream = true
	} else if req.StartTime < s.stream.lastEndTime {
		needNewStream = true
	}

	if needNewStream {
		events, errors := s.clt.StreamSessionEvents(
			metadata.WithSessionRecordingFormatContext(ctx, teleport.PTY),
			session.ID(s.sessionID),
			0,
		)

		if events == nil || errors == nil {
			s.sendError(fmt.Errorf("failed to start session event stream"), req.RequestID)
			s.stream.Unlock()

			return
		}

		s.stream.eventsChan = events
		s.stream.errorsChan = errors
		s.stream.lastEndTime = 0

		s.terminal.Lock()
		s.terminal.vt = vt10x.New()
		s.terminal.Unlock()
	}

	s.stream.lastEndTime = req.EndTime

	s.stream.Unlock()

	go s.streamEvents(ctx, req)
}

func (s *SessionEventsHandler) streamEvents(ctx context.Context, req *fetchRequest) {
	defer func() {
		s.mu.Lock()
		s.activeTask = nil
		s.mu.Unlock()
	}()

	s.sendEvent(EventTypeStart, req.StartTime, nil, req.RequestID)

	screenSent := false
	inTimeRange := false
	var streamStartTime time.Time

	const maxBatchSize = 200

	eventBatch := make([]sessionEvent, 0, maxBatchSize)

	flushBatch := func() {
		if len(eventBatch) == 0 {
			return
		}

		s.sendEventBatch(eventBatch, req.RequestID)
		eventBatch = eventBatch[:0]
	}

	addToBatch := func(eventType byte, timestamp int64, data []byte) {
		eventBatch = append(eventBatch, sessionEvent{eventType, timestamp, data})

		if len(eventBatch) >= maxBatchSize {
			flushBatch()
		}
	}

	for {
		select {
		case <-ctx.Done():
			flushBatch()

			s.sendEvent(EventTypeStop, 0, encodeTime(req.StartTime, req.EndTime), req.RequestID)

			return

		case err := <-s.stream.errorsChan:
			flushBatch()

			if err != nil {
				s.sendError(err, req.RequestID)
			}

			s.sendEvent(EventTypeStop, 0, encodeTime(req.StartTime, req.EndTime), req.RequestID)
			return

		case evt, ok := <-s.stream.eventsChan:
			if !ok {
				flushBatch()

				if req.RequestCurrentScreen && !screenSent && inTimeRange {
					s.sendCurrentScreen(req.RequestID, req.StartTime)
				}

				s.sendEvent(EventTypeStop, 0, encodeTime(req.StartTime, req.EndTime), req.RequestID)
				return
			}

			if _, ok := evt.(*apievents.SessionStart); ok && streamStartTime.IsZero() {
				streamStartTime = evt.GetTime()
			}

			eventTime := getEventTime(evt)

			if !inTimeRange && eventTime >= req.StartTime {
				inTimeRange = true

				if req.RequestCurrentScreen && !screenSent {
					flushBatch()

					s.sendCurrentScreen(req.RequestID, eventTime)

					screenSent = true
				}
			}

			switch evt := evt.(type) {
			case *apievents.SessionStart:
				s.resizeTerminal(evt.TerminalSize)

				if inTimeRange {
					addToBatch(EventTypeSessionStart, 0, []byte(evt.TerminalSize))
				}

			case *apievents.SessionPrint:
				s.terminal.Lock()
				_, _ = s.terminal.vt.Write(evt.Data)
				s.terminal.Unlock()

				if evt.DelayMilliseconds >= req.StartTime && evt.DelayMilliseconds <= req.EndTime {
					addToBatch(EventTypeSessionPrint, evt.DelayMilliseconds, evt.Data)
				} else if evt.DelayMilliseconds > req.EndTime {
					flushBatch()

					s.sendEvent(EventTypeStop, 0, encodeTime(req.StartTime, req.EndTime), req.RequestID)

					return
				}

			case *apievents.SessionEnd:
				endTime := int64(evt.EndTime.Sub(evt.StartTime) / time.Millisecond)

				if endTime >= req.StartTime && endTime <= req.EndTime {
					addToBatch(EventTypeSessionEnd, endTime, []byte(evt.EndTime.Format(time.RFC3339)))
				}

				flushBatch()

				s.sendEvent(EventTypeStop, 0, encodeTime(req.StartTime, req.EndTime), req.RequestID)

				return

			case *apievents.Resize:
				s.resizeTerminal(evt.TerminalSize)

				if inTimeRange {
					addToBatch(EventTypeResize, 0, []byte(evt.TerminalSize))
				}
			}
		}
	}
}

func (s *SessionEventsHandler) resizeTerminal(size string) {
	parts := strings.Split(size, ":")
	if len(parts) != 2 {
		return
	}

	cols, cErr := strconv.Atoi(parts[0])
	rows, rErr := strconv.Atoi(parts[1])

	if cmp.Or(cErr, rErr) != nil {
		return
	}

	s.terminal.Lock()
	defer s.terminal.Unlock()

	s.terminal.vt.Resize(cols, rows)
}

func (s *SessionEventsHandler) WriteMessage(msgType int, data []byte) error {
	s.websocket.Lock()
	defer s.websocket.Unlock()

	if err := s.websocket.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}

	return s.websocket.WriteMessage(msgType, data)
}

func (s *SessionEventsHandler) sendEvent(eventType byte, timestamp int64, data []byte, requestID int) {
	buf := make([]byte, responseHeaderSize+len(data))
	buf[0] = eventType

	binary.BigEndian.PutUint64(buf[1:9], uint64(timestamp))
	binary.BigEndian.PutUint32(buf[9:13], uint32(len(data)))
	binary.BigEndian.PutUint32(buf[13:17], uint32(requestID))

	copy(buf[17:], data)

	if err := s.WriteMessage(websocket.BinaryMessage, buf); err != nil {
		s.logger.WarnContext(s.ctx, "failed to send event", "error", err)
	}
}

func (s *SessionEventsHandler) sendEventBatch(batch []sessionEvent, requestID int) {
	eventsSize := 0
	for _, evt := range batch {
		eventsSize += 17 + len(evt.data)
	}

	buf := make([]byte, responseHeaderSize+eventsSize)

	buf[0] = EventTypeBatch

	binary.BigEndian.PutUint32(buf[1:5], uint32(len(batch)))
	binary.BigEndian.PutUint32(buf[5:9], uint32(requestID))

	// keep the header size the same for all events
	offset := 17

	for _, evt := range batch {
		buf[offset] = evt.eventType

		binary.BigEndian.PutUint64(buf[offset+1:offset+9], uint64(evt.timestamp))
		binary.BigEndian.PutUint32(buf[offset+9:offset+13], uint32(len(evt.data)))
		binary.BigEndian.PutUint32(buf[offset+13:offset+17], uint32(requestID))

		copy(buf[offset+17:], evt.data)

		offset += 17 + len(evt.data)
	}

	if err := s.WriteMessage(websocket.BinaryMessage, buf); err != nil {
		s.logger.WarnContext(s.ctx, "failed to send event batch", "error", err, "batch_size", len(batch))
	}
}

func (s *SessionEventsHandler) sendError(err error, requestID int) {
	s.sendEvent(EventTypeError, 0, []byte(err.Error()), requestID)
}

func (s *SessionEventsHandler) sendCurrentScreen(requestID int, timestamp int64) {
	s.terminal.RLock()
	defer s.terminal.RUnlock()

	data := encodeScreenEvent(s.terminal.vt)

	s.sendEvent(EventTypeScreen, timestamp, data, requestID)
}

func encodeScreenEvent(vt vt10x.Terminal) []byte {
	cols, rows := vt.Size()
	cursor := vt.Cursor()
	data := terminalStateToANSI(vt.DumpState())

	eventData := make([]byte, 21+len(data))
	eventData[0] = EventTypeScreen

	binary.BigEndian.PutUint32(eventData[1:5], uint32(cols))
	binary.BigEndian.PutUint32(eventData[5:9], uint32(rows))
	binary.BigEndian.PutUint32(eventData[9:13], uint32(cursor.X))
	binary.BigEndian.PutUint32(eventData[13:17], uint32(cursor.Y))
	binary.BigEndian.PutUint32(eventData[17:21], uint32(len(data)))

	copy(eventData[21:], data)

	return eventData
}

func encodeTime(startTime, endTime int64) []byte {
	buf := make([]byte, 16)

	binary.BigEndian.PutUint64(buf, uint64(startTime))
	binary.BigEndian.PutUint64(buf[8:], uint64(endTime))

	return buf
}

func decodeBinaryRequest(data []byte) (*fetchRequest, error) {
	if len(data) < requestHeaderSize {
		return nil, fmt.Errorf("request too short")
	}

	req := &fetchRequest{
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

func validateRequest(req *fetchRequest) error {
	if req.StartTime < 0 || req.EndTime < 0 {
		return fmt.Errorf("invalid time range")
	}

	if req.EndTime < req.StartTime {
		return fmt.Errorf("end time before start time")
	}

	rangeMillis := req.EndTime - req.StartTime
	maxRangeMillis := int64(maxRequestRange / time.Millisecond)

	if rangeMillis > maxRangeMillis {
		return fmt.Errorf("time range too large")
	}

	return nil
}
