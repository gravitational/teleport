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
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
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
	// EventTypeLineChange indicates a line change event
	EventTypeLineChange byte = 9
	// EventTypeResizeWithChanges indicates a resize event with line changes
	EventTypeResizeWithChanges byte = 10
	// EventTypeScreenWithChanges indicates a full screen state with all lines
	EventTypeScreenWithChanges byte = 11

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

type screenCapture struct {
	primaryBuf  bytes.Buffer
	altBuf      bytes.Buffer
	inAltScreen bool
	writer      io.Writer
}

func newScreenCapture(w io.Writer) *screenCapture {
	return &screenCapture{
		writer: w,
	}
}

func (sc *screenCapture) Write(p []byte) (n int, err error) {
	// Always pass through to original writer
	n, err = sc.writer.Write(p)
	if err != nil {
		return n, err
	}

	// Capture to appropriate buffer
	buf := &sc.primaryBuf
	if sc.inAltScreen {
		buf = &sc.altBuf
	}

	// Parse for mode changes
	remaining := p
	for len(remaining) > 0 {
		if idx := bytes.Index(remaining, []byte("\x1b[")); idx != -1 {
			// Write everything before escape sequence
			if idx > 0 {
				buf.Write(remaining[:idx])
			}

			// Check if this is a mode change
			if bytes.HasPrefix(remaining[idx:], []byte("\x1b[?1049h")) {
				sc.inAltScreen = true
				buf = &sc.altBuf
				remaining = remaining[idx+8:]
			} else if bytes.HasPrefix(remaining[idx:], []byte("\x1b[?1049l")) {
				sc.inAltScreen = false
				buf = &sc.primaryBuf
				remaining = remaining[idx+8:]
			} else {
				// Not a mode change, write the escape char and continue
				buf.WriteByte(remaining[idx])
				remaining = remaining[idx+1:]
			}
		} else {
			// No more escape sequences
			buf.Write(remaining)
			break
		}
	}

	return n, nil
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

type lineSegments struct {
	Line     int
	Segments []vt10x.Segment
}

func encodeTerminalChanges(lines []lineSegments, cursorVisible bool, cursorX, cursorY int) []byte {
	return encodeTerminalChangesInternal(lines, cursorVisible, cursorX, cursorY, nil)
}

func encodeResizeWithChanges(lines []lineSegments, cursorVisible bool, cursorX, cursorY, cols, rows int) []byte {
	size := &terminalSize{cols: cols, rows: rows}
	return encodeTerminalChangesInternal(lines, cursorVisible, cursorX, cursorY, size)
}

func encodeScreenWithChanges(vt vt10x.Terminal) []byte {
	cols, rows := vt.Size()
	cursor := vt.Cursor()
	cursorVisible := vt.CursorVisible()

	// Get all lines
	lines := make([]lineSegments, 0, rows)
	for i := 0; i < rows; i++ {
		segments := vt.Line(i)
		if len(segments) > 0 {
			lines = append(lines, lineSegments{
				Line:     i,
				Segments: segments,
			})
		}
	}

	size := &terminalSize{cols: cols, rows: rows}
	return encodeTerminalChangesInternal(lines, cursorVisible, cursor.X, cursor.Y, size)
}

type terminalSize struct {
	cols, rows int
}

func encodeTerminalChangesInternal(lines []lineSegments, cursorVisible bool, cursorX, cursorY int, size *terminalSize) []byte {
	totalSize := 4 + 1 + 4 + 4 // line count + cursor visible + cursor X + cursor Y

	if size != nil {
		totalSize += 8 // cols + rows
	}

	for _, ls := range lines {
		totalSize += 8
		for _, seg := range ls.Segments {
			textBytes := []byte(seg.Text)
			totalSize += 4 + len(textBytes)
			totalSize += 1
			totalSize += 8
			totalSize += 12
			totalSize += 1

			if seg.ExtraClass != nil {
				extraBytes := []byte(*seg.ExtraClass)
				totalSize += 4 + len(extraBytes)
			}
		}
	}

	buf := make([]byte, totalSize)
	offset := 0

	binary.BigEndian.PutUint32(buf[offset:], uint32(len(lines)))
	offset += 4

	// Add cursor information
	if cursorVisible {
		buf[offset] = 1
	} else {
		buf[offset] = 0
	}
	offset += 1

	binary.BigEndian.PutUint32(buf[offset:], uint32(cursorX))
	offset += 4

	binary.BigEndian.PutUint32(buf[offset:], uint32(cursorY))
	offset += 4

	// Add terminal size if provided
	if size != nil {
		binary.BigEndian.PutUint32(buf[offset:], uint32(size.cols))
		offset += 4
		binary.BigEndian.PutUint32(buf[offset:], uint32(size.rows))
		offset += 4
	}

	for _, ls := range lines {
		offset = encodeLineSegments(buf, offset, ls.Line, ls.Segments)
	}

	return buf[:offset]
}

func encodeLineSegments(buf []byte, offset int, line int, segments []vt10x.Segment) int {
	binary.BigEndian.PutUint32(buf[offset:], uint32(line))
	offset += 4
	binary.BigEndian.PutUint32(buf[offset:], uint32(len(segments)))
	offset += 4

	for _, seg := range segments {
		textBytes := []byte(seg.Text)

		binary.BigEndian.PutUint32(buf[offset:], uint32(len(textBytes)))
		offset += 4
		copy(buf[offset:], textBytes)
		offset += len(textBytes)

		penFlags := byte(0)
		if seg.Pen.IsBold {
			penFlags |= 1 << 0
		}
		if seg.Pen.IsItalic {
			penFlags |= 1 << 1
		}
		if seg.Pen.IsUnderline {
			penFlags |= 1 << 2
		}
		if seg.Pen.IsBlink {
			penFlags |= 1 << 3
		}
		if seg.Pen.IsInverse {
			penFlags |= 1 << 4
		}
		buf[offset] = penFlags
		offset++

		offset += writePenColor(buf[offset:], seg.Pen.Background)
		offset += writePenColor(buf[offset:], seg.Pen.Foreground)

		binary.BigEndian.PutUint32(buf[offset:], uint32(seg.Offset))
		offset += 4
		binary.BigEndian.PutUint32(buf[offset:], uint32(seg.CellCount))
		offset += 4
		binary.BigEndian.PutUint32(buf[offset:], uint32(seg.CharWidth))
		offset += 4

		if seg.ExtraClass != nil {
			buf[offset] = 1
			offset++
			extraBytes := []byte(*seg.ExtraClass)
			binary.BigEndian.PutUint32(buf[offset:], uint32(len(extraBytes)))
			offset += 4
			copy(buf[offset:], extraBytes)
			offset += len(extraBytes)
		} else {
			buf[offset] = 0
			offset++
		}
	}

	return offset
}

func writePenColor(buf []byte, color *vt10x.PenColor) int {
	if color == nil {
		buf[0] = 0
		return 1
	}

	switch color.Type {
	case "Indexed":
		buf[0] = 1
		buf[1] = color.Value
		return 2
	case "RGB":
		buf[0] = 2
		if color.RGB != nil {
			buf[1] = color.RGB.R
			buf[2] = color.RGB.G
			buf[3] = color.RGB.B
		} else {
			buf[1] = 0
			buf[2] = 0
			buf[3] = 0
		}
		return 4
	default:
		buf[0] = 0
		return 1
	}
}

func (s *SessionEventsHandler) handleFetchRequest(req *BinaryRequest) {
	// Cancel any active streaming task
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

	// Always start streaming from the beginning
	events, errors := s.clt.StreamSessionEvents(
		metadata.WithSessionRecordingFormatContext(ctx, teleport.PTY),
		session.ID(s.sessionID),
		0,
	)

	vt := vt10x.New()
	currentTime := int64(0)
	index := 0
	screenSent := false
	inTimeRange := false
	var streamStartTime time.Time

	// Stream events
	for {
		select {
		case <-ctx.Done():
			s.sendEvent(EventTypeStop, 0, encodeTime(req.StartTime, req.EndTime), 0, req.RequestID)
			return
		case err := <-errors:
			if err != nil {
				s.sendError(err, req.RequestID)
			}
			s.sendEvent(EventTypeStop, 0, encodeTime(req.StartTime, req.EndTime), 0, req.RequestID)
			return
		case evt, ok := <-events:
			if !ok {
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

			// Check if we've entered the requested time range
			if !inTimeRange && eventTime >= req.StartTime {
				inTimeRange = true
				// Send screen state when entering the time range
				if req.RequestCurrentScreen && !screenSent {
					s.sendScreenState(vt, eventTime, req.RequestID)
					screenSent = true
				}
			}

			// Process the event
			switch evt := evt.(type) {
			case *apievents.SessionStart:
				resizeTerminal(vt, evt.TerminalSize)
				if inTimeRange {
					s.sendEvent(EventTypeSessionStart, 0, []byte(evt.TerminalSize), index, req.RequestID)
					index++
				}

			case *apievents.SessionPrint:
				// Always update terminal state
				_, linesChanged, _ := vt.WriteWithChanges(evt.Data)

				currentTime = evt.DelayMilliseconds

				// Only send events within the requested time range
				if evt.DelayMilliseconds >= req.StartTime && evt.DelayMilliseconds <= req.EndTime {
					lines := make([]lineSegments, 0, len(linesChanged))

					for _, line := range linesChanged {
						segments := vt.Line(line)

						lines = append(lines, lineSegments{
							Line:     line,
							Segments: segments,
						})
					}

					// Get cursor information
					cursor := vt.Cursor()
					cursorVisible := vt.CursorVisible()

					s.sendEvent(EventTypeLineChange, evt.DelayMilliseconds, encodeTerminalChanges(lines, cursorVisible, cursor.X, cursor.Y), index, req.RequestID)
					//s.sendEvent(EventTypeSessionPrint, evt.DelayMilliseconds, evt.Data, index, req.RequestID)
					index++
				} else if evt.DelayMilliseconds > req.EndTime {
					// We've passed the end time, stop streaming
					s.sendEvent(EventTypeStop, 0, encodeTime(req.StartTime, req.EndTime), 0, req.RequestID)
					return
				}

			case *apievents.SessionEnd:
				endTime := int64(evt.EndTime.Sub(evt.StartTime) / time.Millisecond)
				if endTime >= req.StartTime && endTime <= req.EndTime {
					s.sendEvent(EventTypeSessionEnd, endTime, []byte(evt.EndTime.Format(time.RFC3339)), index, req.RequestID)
				}
				s.sendEvent(EventTypeStop, 0, encodeTime(req.StartTime, req.EndTime), 0, req.RequestID)
				return

			case *apievents.Resize:
				linesChanged := resizeTerminal(vt, evt.TerminalSize)
				if inTimeRange {
					lines := make([]lineSegments, 0, len(linesChanged))
					for _, line := range linesChanged {
						segments := vt.Line(line)
						lines = append(lines, lineSegments{
							Line:     line,
							Segments: segments,
						})
					}

					cols, rows := vt.Size()
					cursor := vt.Cursor()
					cursorVisible := vt.CursorVisible()

					// use the start timestamp to get the resize time
					var timestamp int64
					if !streamStartTime.IsZero() {
						timestamp = evt.Time.Sub(streamStartTime).Milliseconds()
					}

					// Send resize event with changes
					s.sendEvent(EventTypeResizeWithChanges, timestamp, encodeResizeWithChanges(lines, cursorVisible, cursor.X, cursor.Y, cols, rows), index, req.RequestID)

					//s.sendEvent(EventTypeResize, 0, []byte(evt.TerminalSize), index, req.RequestID)
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

func (s *SessionEventsHandler) sendError(err error, requestID int) {
	s.sendEvent(EventTypeError, 0, []byte(err.Error()), 0, requestID)
}

func (s *SessionEventsHandler) sendScreenState(vt vt10x.Terminal, currentTime int64, requestID int) {
	// Send the new screen with changes event
	data := encodeScreenWithChanges(vt)
	s.sendEvent(EventTypeScreenWithChanges, currentTime, data, 0, requestID)
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

func encodeSerializedTerminal(screen *terminal.SerializedTerminal) []byte {
	dataBytes := []byte(screen.Data)
	buf := make([]byte, 20+len(dataBytes))

	binary.BigEndian.PutUint32(buf[0:4], uint32(screen.Cols))
	binary.BigEndian.PutUint32(buf[4:8], uint32(screen.Rows))
	binary.BigEndian.PutUint32(buf[8:12], uint32(screen.CursorX))
	binary.BigEndian.PutUint32(buf[12:16], uint32(screen.CursorY))
	binary.BigEndian.PutUint32(buf[16:20], uint32(len(dataBytes)))
	copy(buf[20:], dataBytes)

	return buf
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

func resizeTerminal(vt vt10x.Terminal, size string) []int {
	parts := strings.Split(size, ":")
	if len(parts) != 2 {
		return nil
	}

	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil
	}

	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil
	}

	return vt.ResizeWithChanges(width, height)
}
