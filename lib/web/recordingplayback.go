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
	"fmt"
	"log/slog"
	"net/http"
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

type requestType byte

// Identifies requests coming from the client (web UI)
const (
	// requestTypeFetch requests event data
	requestTypeFetch requestType = 1
)

type responseType byte

// Response types send back to the client
const (
	// eventTypeStart indicates the start of a response of events
	eventTypeStart responseType = 1
	// eventTypeStop indicates the stop of a response of events
	eventTypeStop responseType = 2
	// eventTypeError indicates an error
	eventTypeError responseType = 3
	// eventTypeSessionStart indicates session started
	eventTypeSessionStart responseType = 4
	// eventTypeSessionPrint contains terminal output
	eventTypeSessionPrint responseType = 5
	// eventTypeSessionEnd indicates session ended
	eventTypeSessionEnd responseType = 6
	// eventTypeResize indicates terminal resize
	eventTypeResize responseType = 7
	// eventTypeScreen contains terminal screen state
	eventTypeScreen responseType = 8
	// eventTypeBatch indicates a batch of events
	eventTypeBatch responseType = 9
)

const (
	// maxRequestRange is the maximum allowed time range for a request
	maxRequestRange = 10 * time.Minute
	// requestHeaderSize is the size of the request header (event type, start time, end time, request ID, and current screen flag)
	requestHeaderSize = 22
	// responseHeaderSize is the size of the response header (event type, timestamp, data size, and request ID)
	responseHeaderSize = 17
)

const websocketCloseTimeout = 5 * time.Second

// websocketMessage represents a message to be written to the websocket
type websocketMessage struct {
	messageType int
	data        []byte
}

// recordingPlayback manages session event streaming
type recordingPlayback struct {
	ctx              context.Context
	cancel           context.CancelFunc
	clt              events.SessionStreamer
	sessionID        string
	logger           *slog.Logger
	mu               sync.Mutex
	cancelActiveTask context.CancelFunc
	taskWg           *sync.WaitGroup
	ws               *websocket.Conn
	writeChan        chan websocketMessage
	closeSent        bool // tracks if a close message has been sent

	stream struct {
		sync.Mutex
		eventsChan    <-chan apievents.AuditEvent
		errorsChan    <-chan error
		lastEndTime   time.Duration
		bufferedEvent apievents.AuditEvent
	}

	terminal struct {
		sync.Mutex
		vt          vt10x.Terminal
		currentTime int64
	}
}

// fetchRequest represents a request for session events.
type fetchRequest struct {
	requestType          requestType
	startOffset          time.Duration
	endOffset            time.Duration
	requestID            int
	requestCurrentScreen bool
}

// sessionEvent represents a single session event with its type, timestamp, and data.
type sessionEvent struct {
	eventType  responseType
	timeOffset time.Duration
	data       []byte
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

		gracefulWebSocketClose(ws, websocketCloseTimeout)

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
		ws:        ws,
		writeChan: make(chan websocketMessage),
	}

	return s
}

// run starts the recording playback handler.
func (s *recordingPlayback) run() {
	defer s.cleanup()

	go s.writeLoop()

	s.readLoop()
}

// cleanup cleans up the recording playback resources.
func (s *recordingPlayback) cleanup() {
	s.cancel()

	// Wait for any active task to complete
	s.mu.Lock()
	wg := s.taskWg
	alreadySent := s.closeSent
	s.mu.Unlock()

	if wg != nil {
		wg.Wait()
	}

	// Only send close message if we haven't already sent one
	if !alreadySent {
		select {
		case s.writeChan <- websocketMessage{
			messageType: websocket.CloseMessage,
			data:        websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		}:
		case <-time.After(websocketCloseTimeout):
		}
	}

	close(s.writeChan)

	// Wait for peer's close frame response (or timeout)
	deadline := time.Now().Add(websocketCloseTimeout)
	_ = s.ws.SetReadDeadline(deadline)
	_, _, _ = s.ws.ReadMessage()

	// Finally close the underlying connection
	s.ws.Close()
}

// writeLoop handles all websocket writes from a dedicated goroutine.
func (s *recordingPlayback) writeLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case msg, ok := <-s.writeChan:
			if !ok {
				return
			}

			if err := s.ws.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				s.logWebsocketError(err)
				return
			}

			if err := s.ws.WriteMessage(msg.messageType, msg.data); err != nil {
				s.logWebsocketError(err)
				return
			}

			// If we just sent a close message, exit the loop
			if msg.messageType == websocket.CloseMessage {
				return
			}
		}
	}
}

// logWebsocketError handles errors that occur during websocket writes.
func (s *recordingPlayback) logWebsocketError(err error) {
	if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) &&
		!utils.IsOKNetworkError(err) {
		s.logger.ErrorContext(s.ctx, "websocket write error", "error", err)
	}
}

// readLoop reads messages from the websocket connection and processes them.
func (s *recordingPlayback) readLoop() {
	for {
		msgType, data, err := s.ws.ReadMessage()
		if err != nil {
			s.logWebsocketError(err)
			return
		}

		if msgType != websocket.BinaryMessage {
			s.logger.ErrorContext(s.ctx, "ignoring non-binary websocket message", "session_id", s.sessionID, "type", msgType)

			// Mark that we're sending a close message
			s.mu.Lock()
			s.closeSent = true
			s.mu.Unlock()

			// Send close message through the write channel
			select {
			case s.writeChan <- websocketMessage{
				messageType: websocket.CloseMessage,
				data:        websocket.FormatCloseMessage(websocket.CloseUnsupportedData, "only binary messages are supported"),
			}:
			default:
			}

			return
		}

		req, err := decodeBinaryRequest(data)
		if err != nil {
			s.logger.ErrorContext(s.ctx, "failed to decode request", "session_id", s.sessionID, "error", err)
			continue
		}

		switch req.requestType {
		case requestTypeFetch:
			s.handleFetchRequest(req)
		default:
			s.sendError(trace.BadParameter("unknown request type: %d", req.requestType), req.requestID)

			s.logger.ErrorContext(s.ctx, "received unknown request type", "session_id", s.sessionID, "type", req.requestType)
		}
	}
}

// createTaskContext creates a new context for a task and cancels any previous task.
// A task context is used to manage the lifecycle of a fetch request, ensuring that only one fetch request is active at a time.
func (s *recordingPlayback) createTaskContext() context.Context {
	s.mu.Lock()

	if s.cancelActiveTask != nil {
		// Cancel the active task first
		s.cancelActiveTask()
		oldWg := s.taskWg
		s.mu.Unlock()

		// Wait for streamEvents to terminate before continuing
		// We unlock the mutex while waiting to avoid deadlock
		if oldWg != nil {
			oldWg.Wait()
		}

		s.mu.Lock()
	}

	// Create a new WaitGroup for the new task
	s.taskWg = &sync.WaitGroup{}

	ctx, taskCancel := context.WithCancel(s.ctx)
	s.cancelActiveTask = taskCancel
	s.mu.Unlock()

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

	// start the stream if it doesn't exist or if we need to go back in time
	needNewStream := s.stream.eventsChan == nil || req.startOffset < s.stream.lastEndTime

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

	eventsChan := s.stream.eventsChan
	errorsChan := s.stream.errorsChan

	s.stream.Unlock()

	// Get the current task's WaitGroup
	s.mu.Lock()
	wg := s.taskWg
	s.mu.Unlock()

	if wg != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.streamEvents(ctx, req, eventsChan, errorsChan)
		}()
	} else {
		go s.streamEvents(ctx, req, eventsChan, errorsChan)
	}
}

// streamEvents streams session events to the client.
func (s *recordingPlayback) streamEvents(ctx context.Context, req *fetchRequest, eventsChan <-chan apievents.AuditEvent, errorsChan <-chan error) {

	startSent := false
	screenSent := false
	inTimeRange := false
	var streamStartTime time.Time

	const maxBatchSize = 200
	eventBatch := make([]sessionEvent, 0, maxBatchSize)

	flushBatch := func() {
		// Send start event if not already sent
		if !startSent {
			s.sendEvent(eventTypeStart, req.startOffset, nil, req.requestID)
			startSent = true
		}

		if len(eventBatch) == 0 {
			return
		}

		s.sendEventBatch(eventBatch, req.requestID)
		eventBatch = eventBatch[:0]
	}

	addToBatch := func(eventType responseType, timeOffset time.Duration, data []byte) {
		eventBatch = append(eventBatch, sessionEvent{eventType, timeOffset, data})

		if len(eventBatch) >= maxBatchSize {
			flushBatch()
		}
	}

	sendStop := func() {
		// Send start event if not already sent
		if !startSent {
			s.sendEvent(eventTypeStart, req.startOffset, nil, req.requestID)
			startSent = true
		}

		s.sendEvent(eventTypeStop, 0, encodeTime(req.startOffset, req.endOffset), req.requestID)
	}

	// process an event, returning a boolean indicating if the events should continue being
	// processed (i.e. returns false once we have reached the end of the requested time window)
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

		if eventTime > req.endOffset {
			s.stream.Lock()
			// store the event for the next request as it is outside the current time range
			// and won't be returned by the stream on the next request
			// this will only store print or end events as they are the only ones with a timestamp
			s.stream.bufferedEvent = evt
			s.stream.Unlock()

			return false
		}

		switch evt := evt.(type) {
		case *apievents.SessionStart:
			if err := s.resizeTerminal(evt.TerminalSize); err != nil {
				s.logger.ErrorContext(s.ctx, "failed to resize terminal", "session_id", s.sessionID, "error", err)

				// continue returning events even if resize fails
			}

			if inTimeRange {
				addToBatch(eventTypeSessionStart, 0, []byte(evt.TerminalSize))
			}

		case *apievents.SessionPrint:
			s.terminal.Lock()
			if _, err := s.terminal.vt.Write(evt.Data); err != nil {
				s.logger.ErrorContext(s.ctx, "failed to write to terminal", "session_id", s.sessionID, "error", err)
			}
			s.terminal.Unlock()

			if inTimeRange {
				addToBatch(eventTypeSessionPrint, eventTime, evt.Data)
			}

		case *apievents.SessionEnd:
			endTime := evt.EndTime.Sub(evt.StartTime)

			if inTimeRange {
				addToBatch(eventTypeSessionEnd, endTime, []byte(evt.EndTime.Format(time.RFC3339)))
			}

			return false

		case *apievents.Resize:
			if err := s.resizeTerminal(evt.TerminalSize); err != nil {
				s.logger.ErrorContext(s.ctx, "failed to resize terminal", "session_id", s.sessionID, "error", err)

				// continue returning events even if resize fails
			}

			// always add resize events as they do not have a timestamp
			addToBatch(eventTypeResize, 0, []byte(evt.TerminalSize))
		}

		return true
	}

	s.stream.Lock()

	buffered := s.stream.bufferedEvent
	s.stream.bufferedEvent = nil

	s.stream.Unlock()

	if buffered != nil {
		// process any buffered event from a previous request first
		// the processEvent will ignore it if it's outside the requested time range
		_ = processEvent(buffered)
	}

	for {
		select {
		case <-ctx.Done():
			// Don't send any more events after context cancellation
			return

		case err := <-errorsChan:
			flushBatch()
			if err != nil {
				s.sendError(err, req.requestID)
			}
			sendStop()

			return

		case evt, ok := <-eventsChan:
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
func (s *recordingPlayback) resizeTerminal(size string) error {
	params, err := session.UnmarshalTerminalParams(size)
	if err != nil {
		return trace.Wrap(err)
	}

	s.terminal.Lock()
	defer s.terminal.Unlock()

	s.terminal.vt.Resize(params.W, params.H)

	return nil
}

// writeMessage sends a message through the write channel.
func (s *recordingPlayback) writeMessage(data []byte) error {
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	case s.writeChan <- websocketMessage{messageType: websocket.BinaryMessage, data: data}:
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout sending message")
	}
}

// sendEvent sends a single event to the client.
func (s *recordingPlayback) sendEvent(eventType responseType, timeOffset time.Duration, data []byte, requestID int) {
	totalSize := responseHeaderSize + len(data)
	buf := make([]byte, totalSize)

	encodeEvent(buf, 0, eventType, timeOffset, data, requestID)

	if err := s.writeMessage(buf); err != nil {
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

	buf[0] = byte(eventTypeBatch)
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(batch)))
	binary.BigEndian.PutUint32(buf[5:9], uint32(requestID))

	offset := responseHeaderSize
	for _, evt := range batch {
		encodeEvent(buf, offset, evt.eventType, evt.timeOffset, evt.data, requestID)

		offset += responseHeaderSize + len(evt.data)
	}

	if err := s.writeMessage(buf); err != nil {
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
func (s *recordingPlayback) sendCurrentScreen(requestID int, timeOffset time.Duration) {
	s.terminal.Lock()
	state := s.terminal.vt.DumpState()
	cols, rows := s.terminal.vt.Size()
	cursor := s.terminal.vt.Cursor()
	s.terminal.Unlock()

	data := encodeScreenEvent(state, cols, rows, cursor)

	s.sendEvent(eventTypeScreen, timeOffset, data, requestID)
}

// encodeScreenEvent encodes the current terminal screen state into a byte slice.
func encodeScreenEvent(state vt10x.TerminalState, cols, rows int, cursor vt10x.Cursor) []byte {
	data := terminal.VtStateToANSI(state)

	eventData := make([]byte, requestHeaderSize+len(data))
	eventData[0] = byte(eventTypeScreen)

	binary.BigEndian.PutUint32(eventData[1:5], uint32(cols))
	binary.BigEndian.PutUint32(eventData[5:9], uint32(rows))
	binary.BigEndian.PutUint32(eventData[9:13], uint32(cursor.X))
	binary.BigEndian.PutUint32(eventData[13:17], uint32(cursor.Y))
	binary.BigEndian.PutUint32(eventData[17:21], uint32(len(data)))

	copy(eventData[requestHeaderSize:], data)

	return eventData
}

// encodeEvent encodes a session event into a byte slice.
func encodeEvent(buf []byte, offset int, eventType responseType, timeOffset time.Duration, data []byte, requestID int) {
	buf[offset] = byte(eventType)

	binary.BigEndian.PutUint64(buf[offset+1:offset+9], uint64(timeOffset))
	binary.BigEndian.PutUint32(buf[offset+9:offset+13], uint32(len(data)))
	binary.BigEndian.PutUint32(buf[offset+13:offset+17], uint32(requestID))

	copy(buf[offset+responseHeaderSize:], data)
}

// encodeTime encodes the start and end times into a byte slice.
func encodeTime(startTime, endTime time.Duration) []byte {
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
		requestType:          requestType(data[0]),
		startOffset:          time.Duration(binary.BigEndian.Uint64(data[1:9])),
		endOffset:            time.Duration(binary.BigEndian.Uint64(data[9:17])),
		requestID:            int(binary.BigEndian.Uint32(data[17:21])),
		requestCurrentScreen: data[21] == 1,
	}

	return req, nil
}

// getEventTime extracts the event time from an AuditEvent.
func getEventTime(evt apievents.AuditEvent) time.Duration {
	switch evt := evt.(type) {
	case *apievents.SessionPrint:
		return time.Duration(evt.DelayMilliseconds)
	case *apievents.SessionEnd:
		return evt.EndTime.Sub(evt.StartTime)
	default:
		return 0
	}
}

// gracefulWebSocketClose performs the proper WebSocket close handshake (RFC 6455).
// This ensures all frames are delivered before connection termination and prevents data
// loss in fast WebSocket connections.
func gracefulWebSocketClose(ws *websocket.Conn, timeout time.Duration) {
	deadline := time.Now().Add(timeout)

	// Send close frame to initiate graceful shutdown
	_ = ws.SetWriteDeadline(deadline)
	_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

	// Wait for peer's close frame response (or timeout)
	_ = ws.SetReadDeadline(deadline)
	_, _, _ = ws.ReadMessage()

	// Finally close the underlying connection
	ws.Close()
}

// validateRequest validates the fetch request parameters.
func validateRequest(req *fetchRequest) error {
	if req.startOffset < 0 || req.endOffset < 0 || req.endOffset < req.startOffset {
		return fmt.Errorf("invalid time range (%d, %d)", req.startOffset, req.endOffset)
	}

	rangeMillis := req.endOffset - req.startOffset
	maxRangeMillis := time.Duration(maxRequestRange / time.Millisecond)

	if rangeMillis > maxRangeMillis {
		return trace.LimitExceeded("time range too large, max is %s", maxRequestRange)
	}

	return nil
}
