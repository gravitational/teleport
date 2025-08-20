/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"cmp"
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/hinshun/vt10x"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/metadata"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/terminal"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// requestTypeFetch requests event data
	requestTypeFetch byte = 1
	// requestTypeClose closes the connection
	requestTypeClose byte = 2

	// eventTypeStart indicates stream start
	eventTypeStart byte = 1
	// eventTypeStop indicates stream stop
	eventTypeStop byte = 2
	// eventTypeError indicates an error
	eventTypeError byte = 3
	// eventTypeSessionStart indicates session started
	eventTypeSessionStart byte = 4
	// eventTypeSessionPrint contains terminal output
	eventTypeSessionPrint byte = 5
	// eventTypeSessionEnd indicates session ended
	eventTypeSessionEnd byte = 6
	// eventTypeResize indicates terminal resize
	eventTypeResize byte = 7
	// eventTypeScreen contains terminal screen state
	eventTypeScreen byte = 8
	// eventTypeBatch indicates a batch of events
	eventTypeBatch byte = 9
)

const (
	// maxRequestRange is the maximum allowed time range for a request
	maxRequestRange = 10 * time.Minute
	// requestHeaderSize is the size of the request header (event type, start time, end time, request ID, and current screen flag)
	requestHeaderSize = 22
	// responseHeaderSize is the size of the response header (event type, timestamp, data size, and request ID)
	responseHeaderSize = 17
)

// recordingPlayback manages session event streaming
type recordingPlayback struct {
	ctx        context.Context
	cancel     context.CancelFunc
	clt        events.SessionStreamer
	sessionID  string
	logger     *slog.Logger
	mu         sync.Mutex
	activeTask context.CancelFunc

	stream struct {
		sync.Mutex
		eventsChan     <-chan apievents.AuditEvent
		errorsChan     <-chan error
		lastEndTime    int64
		bufferedEvents []apievents.AuditEvent
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

// fetchRequest represents a request for session events.
type fetchRequest struct {
	requestType          byte
	startOffset          int64
	endOffset            int64
	requestID            int
	requestCurrentScreen bool
}

// sessionEvent represents a single session event with its type, timestamp, and data.
type sessionEvent struct {
	eventType byte
	timestamp int64
	data      []byte
}

func (h *Handler) recordingPlaybackWs(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sctx *SessionContext,
	cluster reversetunnelclient.Cluster,
	ws *websocket.Conn,
) (interface{}, error) {
	sessionID := p.ByName("session_id")
	if sessionID == "" {
		return nil, trace.BadParameter("missing session ID in request URL")
	}

	defer ws.Close()

	ctx := r.Context()
	clt, err := sctx.GetUserClient(ctx, cluster)
	if err != nil {
		data := []byte(err.Error())

		totalSize := responseHeaderSize + len(data)
		buf := make([]byte, totalSize)

		encodeEvent(buf, 0, eventTypeError, 0, data, 0)

		if err := ws.WriteMessage(websocket.BinaryMessage, buf); err != nil {
			h.logger.ErrorContext(ctx, "failed to send event", "session_id", sessionID, "error", err)
		}

		return nil, nil
	}

	playback := newRecordingPlayback(ctx, ws, clt, sessionID, h.logger)

	playback.run()

	return nil, nil
}

// newRecordingPlayback creates a new session recording playback handler.
// This provides a way for the client to request session events within a specific time range, as well as the current
// terminal screen state at a given time (when seeking).
// This allows for faster seeking without having to send the client extra events to reconstruct the terminal state.
func newRecordingPlayback(ctx context.Context, ws *websocket.Conn, clt events.SessionStreamer, sessionID string, logger *slog.Logger) *recordingPlayback {
	ctx, cancel := context.WithCancel(ctx)

	s := &recordingPlayback{
		ctx:       ctx,
		cancel:    cancel,
		clt:       clt,
		sessionID: sessionID,
		logger:    logger,
	}

	s.websocket.Conn = ws

	return s
}

// run starts the recording playback handler.
func (s *recordingPlayback) run() {
	defer s.cleanup()
	s.readLoop()
}

// cleanup cleans up the recording playback resources.
func (s *recordingPlayback) cleanup() {
	s.cancel()

	s.websocket.Lock()
	_ = s.writeMessage(websocket.CloseMessage, nil)
	_ = s.websocket.Close()
	s.websocket.Unlock()
}

// readLoop reads messages from the websocket connection and processes them.
func (s *recordingPlayback) readLoop() {
	for {
		msgType, data, err := s.websocket.ReadMessage()
		if err != nil {
			if !utils.IsOKNetworkError(err) {
				s.logger.ErrorContext(s.ctx, "websocket read error", "session_id", s.sessionID, "error", err)
			}
			return
		}

		if msgType != websocket.BinaryMessage {
			continue
		}

		req, err := decodeBinaryRequest(data)
		if err != nil {
			s.logger.ErrorContext(s.ctx, "failed to decode request", "session_id", s.sessionID, "error", err)
			continue
		}

		switch req.requestType {
		case requestTypeFetch:
			s.handleFetchRequest(req)
		case requestTypeClose:
			return
		}
	}
}

// createTaskContext creates a new context for a task and cancels any previous task.
func (s *recordingPlayback) createTaskContext() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeTask != nil {
		s.activeTask()
	}

	ctx, taskCancel := context.WithCancel(s.ctx)
	s.activeTask = taskCancel

	return ctx
}

// handleFetchRequest processes a fetch request for session events.
func (s *recordingPlayback) handleFetchRequest(req *fetchRequest) {
	if err := validateRequest(req); err != nil {
		s.sendError(err, req.requestID)

		return
	}

	ctx := s.createTaskContext()

	s.stream.Lock()
	needNewStream := false

	if s.stream.eventsChan == nil {
		needNewStream = true
	} else if req.startOffset < s.stream.lastEndTime {
		needNewStream = true
	}

	if needNewStream {
		events, errors := s.clt.StreamSessionEvents(
			metadata.WithSessionRecordingFormatContext(ctx, teleport.PTY),
			session.ID(s.sessionID),
			0,
		)

		if events == nil || errors == nil {
			s.sendError(fmt.Errorf("failed to start session event stream"), req.requestID)
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

	s.stream.lastEndTime = req.endOffset

	s.stream.Unlock()

	go s.streamEvents(ctx, req)
}

// streamEvents streams session events to the client.
func (s *recordingPlayback) streamEvents(ctx context.Context, req *fetchRequest) {
	defer func() {
		s.mu.Lock()
		s.activeTask = nil
		s.mu.Unlock()
	}()

	s.sendEvent(eventTypeStart, req.startOffset, nil, req.requestID)

	screenSent := false
	inTimeRange := false
	var streamStartTime time.Time

	const maxBatchSize = 200
	eventBatch := make([]sessionEvent, 0, maxBatchSize)

	flushBatch := func() {
		if len(eventBatch) == 0 {
			return
		}

		s.sendEventBatch(eventBatch, req.requestID)
		eventBatch = eventBatch[:0]
	}

	addToBatch := func(eventType byte, timestamp int64, data []byte) {
		eventBatch = append(eventBatch, sessionEvent{eventType, timestamp, data})

		if len(eventBatch) >= maxBatchSize {
			flushBatch()
		}
	}

	sendStop := func() {
		s.sendEvent(eventTypeStop, 0, encodeTime(req.startOffset, req.endOffset), req.requestID)
	}

	processEvent := func(evt apievents.AuditEvent) bool {
		if _, ok := evt.(*apievents.SessionStart); ok && streamStartTime.IsZero() {
			streamStartTime = evt.GetTime()
		}

		eventTime := getEventTime(evt)

		if !inTimeRange && eventTime >= req.startOffset {
			inTimeRange = true

			if req.requestCurrentScreen && !screenSent {
				flushBatch()
				s.sendCurrentScreen(req.requestID, eventTime)

				screenSent = true
			}
		}

		switch evt := evt.(type) {
		case *apievents.SessionStart:
			s.resizeTerminal(evt.TerminalSize)

			if inTimeRange {
				addToBatch(eventTypeSessionStart, 0, []byte(evt.TerminalSize))
			}

		case *apievents.SessionPrint:
			if evt.DelayMilliseconds > req.endOffset {
				s.stream.Lock()
				s.stream.bufferedEvents = append(s.stream.bufferedEvents, evt)
				s.stream.Unlock()

				return false
			}

			s.terminal.Lock()
			_, _ = s.terminal.vt.Write(evt.Data)
			s.terminal.Unlock()

			if evt.DelayMilliseconds >= req.startOffset {
				addToBatch(eventTypeSessionPrint, evt.DelayMilliseconds, evt.Data)
			}

		case *apievents.SessionEnd:
			endTime := int64(evt.EndTime.Sub(evt.StartTime) / time.Millisecond)

			if endTime >= req.startOffset && endTime <= req.endOffset {
				addToBatch(eventTypeSessionEnd, endTime, []byte(evt.EndTime.Format(time.RFC3339)))
			}

			flushBatch()
			sendStop()

			return false

		case *apievents.Resize:
			s.resizeTerminal(evt.TerminalSize)

			if inTimeRange {
				addToBatch(eventTypeResize, 0, []byte(evt.TerminalSize))
			}
		}

		return true
	}

	s.stream.Lock()

	buffered := make([]apievents.AuditEvent, len(s.stream.bufferedEvents))
	copy(buffered, s.stream.bufferedEvents)
	s.stream.bufferedEvents = nil

	s.stream.Unlock()

	for _, evt := range buffered {
		if !processEvent(evt) {
			flushBatch()
			sendStop()

			return
		}
	}

	for {
		select {
		case <-ctx.Done():
			flushBatch()
			sendStop()

			return

		case err := <-s.stream.errorsChan:
			flushBatch()
			if err != nil {
				s.sendError(err, req.requestID)
			}
			sendStop()

			return

		case evt, ok := <-s.stream.eventsChan:
			if !ok {
				flushBatch()
				if req.requestCurrentScreen && !screenSent && inTimeRange {
					s.sendCurrentScreen(req.requestID, req.startOffset)
				}
				sendStop()

				return
			}

			if !processEvent(evt) {
				flushBatch()
				sendStop()

				return
			}
		}
	}
}

// resizeTerminal resizes the terminal based on the provided size string.
func (s *recordingPlayback) resizeTerminal(size string) {
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

// writeMessage writes a message to the websocket connection with a timeout.
func (s *recordingPlayback) writeMessage(msgType int, data []byte) error {
	s.websocket.Lock()
	defer s.websocket.Unlock()

	if err := s.websocket.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}

	return s.websocket.WriteMessage(msgType, data)
}

// sendEvent sends a single event to the client.
func (s *recordingPlayback) sendEvent(eventType byte, timestamp int64, data []byte, requestID int) {
	totalSize := responseHeaderSize + len(data)
	buf := make([]byte, totalSize)

	encodeEvent(buf, 0, eventType, timestamp, data, requestID)

	if err := s.writeMessage(websocket.BinaryMessage, buf); err != nil {
		s.logger.ErrorContext(s.ctx, "failed to send event", "session_id", s.sessionID, "error", err)
	}
}

// sendEventBatch sends a batch of events to the client.
func (s *recordingPlayback) sendEventBatch(batch []sessionEvent, requestID int) {
	totalSize := responseHeaderSize
	for _, evt := range batch {
		totalSize += responseHeaderSize + len(evt.data)
	}

	buf := make([]byte, totalSize)

	buf[0] = eventTypeBatch
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(batch)))
	binary.BigEndian.PutUint32(buf[5:9], uint32(requestID))

	offset := responseHeaderSize
	for _, evt := range batch {
		encodeEvent(buf, offset, evt.eventType, evt.timestamp, evt.data, requestID)

		offset += responseHeaderSize + len(evt.data)
	}

	if err := s.writeMessage(websocket.BinaryMessage, buf); err != nil {
		s.logger.ErrorContext(s.ctx, "failed to send event batch",
			"session_id", s.sessionID,
			"error", err,
			"batch_size", len(batch),
			"buffer_size", totalSize)
	}
}

// sendError sends an error event to the client.
func (s *recordingPlayback) sendError(err error, requestID int) {
	s.sendEvent(eventTypeError, 0, []byte(err.Error()), requestID)
}

// sendCurrentScreen sends the current terminal screen state to the client.
func (s *recordingPlayback) sendCurrentScreen(requestID int, timestamp int64) {
	s.terminal.RLock()
	state := s.terminal.vt.DumpState()
	cols, rows := s.terminal.vt.Size()
	cursor := s.terminal.vt.Cursor()
	s.terminal.RUnlock()

	data := encodeScreenEvent(state, cols, rows, cursor)

	s.sendEvent(eventTypeScreen, timestamp, data, requestID)
}

// encodeScreenEvent encodes the current terminal screen state into a byte slice.
func encodeScreenEvent(state vt10x.TerminalState, cols, rows int, cursor vt10x.Cursor) []byte {
	data := terminal.VtStateToANSI(state)

	eventData := make([]byte, requestHeaderSize+len(data))
	eventData[0] = eventTypeScreen

	binary.BigEndian.PutUint32(eventData[1:5], uint32(cols))
	binary.BigEndian.PutUint32(eventData[5:9], uint32(rows))
	binary.BigEndian.PutUint32(eventData[9:13], uint32(cursor.X))
	binary.BigEndian.PutUint32(eventData[13:17], uint32(cursor.Y))
	binary.BigEndian.PutUint32(eventData[17:21], uint32(len(data)))

	copy(eventData[requestHeaderSize:], data)

	return eventData
}

// encodeEvent encodes a session event into a byte slice.
func encodeEvent(buf []byte, offset int, eventType byte, timestamp int64, data []byte, requestID int) {
	buf[offset] = eventType

	binary.BigEndian.PutUint64(buf[offset+1:offset+9], uint64(timestamp))
	binary.BigEndian.PutUint32(buf[offset+9:offset+13], uint32(len(data)))
	binary.BigEndian.PutUint32(buf[offset+13:offset+17], uint32(requestID))

	copy(buf[offset+responseHeaderSize:], data)
}

// encodeTime encodes the start and end times into a byte slice.
func encodeTime(startTime, endTime int64) []byte {
	buf := make([]byte, 16)

	binary.BigEndian.PutUint64(buf, uint64(startTime))
	binary.BigEndian.PutUint64(buf[8:], uint64(endTime))

	return buf
}

// decodeBinaryRequest decodes a binary request from the client.
func decodeBinaryRequest(data []byte) (*fetchRequest, error) {
	if len(data) != requestHeaderSize {
		return nil, trace.BadParameter("invalid request size: expected %d bytes, got %d bytes", requestHeaderSize, len(data))
	}

	req := &fetchRequest{
		requestType:          data[0],
		startOffset:          int64(binary.BigEndian.Uint64(data[1:9])),
		endOffset:            int64(binary.BigEndian.Uint64(data[9:17])),
		requestID:            int(binary.BigEndian.Uint32(data[17:21])),
		requestCurrentScreen: data[21] == 1,
	}

	return req, nil
}

// getEventTime extracts the event time from an AuditEvent.
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

// validateRequest validates the fetch request parameters.
func validateRequest(req *fetchRequest) error {
	if req.startOffset < 0 || req.endOffset < 0 {
		return fmt.Errorf("invalid time range")
	}

	if req.endOffset < req.startOffset {
		return fmt.Errorf("end time before start time")
	}

	rangeMillis := req.endOffset - req.startOffset
	maxRangeMillis := int64(maxRequestRange / time.Millisecond)

	if rangeMillis > maxRangeMillis {
		return fmt.Errorf("time range too large")
	}

	return nil
}
