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

package web

import (
	"bytes"
	"cmp"
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/metadata"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/web/ttyplayback"
	"github.com/gravitational/teleport/lib/web/ttyplayback/terminal"
	"github.com/gravitational/teleport/lib/web/ttyplayback/vt10x"
)

const (
	messageTypePTY       = byte(1)
	messageTypeError     = byte(2)
	messageTypePlayPause = byte(3)
	messageTypeSeek      = byte(4)
	messageTypeResize    = byte(5)
)

const (
	severityError = byte(1)
)

const (
	actionPlay  = byte(0)
	actionPause = byte(1)
)

func (h *Handler) sessionLengthHandle(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sctx *SessionContext,
	site reversetunnelclient.RemoteSite,
) (any, error) {
	sID := p.ByName("sid")
	if sID == "" {
		return nil, trace.BadParameter("missing session ID in request URL")
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	evts, errs := clt.StreamSessionEvents(ctx, session.ID(sID), 0)
	for {
		select {
		case err := <-errs:
			return nil, trace.Wrap(err)
		case evt, ok := <-evts:
			if !ok {
				return nil, trace.NotFound("could not find end event for session %v", sID)
			}
			switch evt := evt.(type) {
			case *apievents.SessionEnd:
				return map[string]any{"durationMs": evt.EndTime.Sub(evt.StartTime).Milliseconds()}, nil
			case *apievents.WindowsDesktopSessionEnd:
				return map[string]any{"durationMs": evt.EndTime.Sub(evt.StartTime).Milliseconds()}, nil
			case *apievents.DatabaseSessionEnd:
				return map[string]any{"durationMs": evt.EndTime.Sub(evt.StartTime).Milliseconds()}, nil
			}
		}
	}
}

func (h *Handler) sessionEvents(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sctx *SessionContext,
	site reversetunnelclient.RemoteSite,
) (interface{}, error) {
	sID := p.ByName("sid")
	if sID == "" {
		return nil, trace.BadParameter("missing session ID in request URL")
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	type Event struct {
		Type string `json:"type"`
		Time int64  `json:"time"`
	}

	var events []Event

	evts, errs := clt.StreamSessionEvents(metadata.WithSessionRecordingFormatContext(ctx, teleport.PTY), session.ID(sID), 0)
	for {
		select {
		case err := <-errs:
			return nil, trace.Wrap(err)
		case evt, ok := <-evts:
			if !ok {
				return events, nil
			}

			switch evt := evt.(type) {
			case *apievents.SessionStart:
				events = append(events, Event{
					Type: "start",
				})
			case *apievents.SessionEnd:
				events = append(events, Event{
					Type: "end",
					Time: evt.EndTime.Sub(evt.StartTime).Milliseconds(),
				})
			case *apievents.Resize:
				events = append(events, Event{
					Type: "resize",
				})
			case *apievents.SessionPrint:
				events = append(events, Event{
					Type: "print",
					Time: evt.DelayMilliseconds,
				})
			}

			if _, isEnd := evt.(*apievents.SessionEnd); isEnd {
				return events, nil
			}
		}
	}
}

func (h *Handler) sessionDetails(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sctx *SessionContext,
	site reversetunnelclient.RemoteSite,
) (interface{}, error) {
	sID := p.ByName("sid")
	if sID == "" {
		return nil, trace.BadParameter("missing session ID in request URL")
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	type Thumbnail struct {
		Screen    terminal.SerializedTerminal `json:"screen"`
		StartTime int64                       `json:"start_time"`
		EndTime   int64                       `json:"end_time"`
	}

	type SessionEvent struct {
		Type      string `json:"type"`
		User      string `json:"user,omitempty"`
		StartTime int64  `json:"start_time"`
		EndTime   int64  `json:"end_time"`
		Cols      int    `json:"cols,omitempty"`
		Rows      int    `json:"rows,omitempty"`
	}

	type SessionEventsResponse struct {
		Duration   int64          `json:"duration"`
		Thumbnails []Thumbnail    `json:"thumbnails"`
		Events     []SessionEvent `json:"events"`
		StartCols  int            `json:"start_cols,omitempty"`
    StartRows  int            `json:"start_rows,omitempty"`
	}

	var sessionStartTime time.Time
	var userEvents []SessionEvent
	activeUsers := make(map[string]int64)

	const inactivityThreshold = 5000
	var lastEventTime time.Time
	var lastEventTimeMs int64
	var startCols int
	var startRows int

	// Initialize VT10X terminal for real-time processing
	vt := vt10x.New()
	var thumbnails []Thumbnail
	const intervalMs = 500 // 200 seconds
	var nextThumbnailTime int64 = 0
	var dataBuffer []byte
	var eventCount int
	var totalDataSize int
	var skippedDataSize int

	// For high-volume sessions, we'll limit data processing
	const maxDataPerThumbnail = 1024 * 1024 // 1MB per thumbnail interval

	evts, errs := clt.StreamSessionEvents(metadata.WithSessionRecordingFormatContext(ctx, teleport.PTY), session.ID(sID), 0)
	for {
		select {
		case err := <-errs:
			return nil, trace.Wrap(err)
		case evt, ok := <-evts:
			if !ok {
				return nil, trace.NotFound("could not find end event for session %v", sID)
			}

			var currentEventTime time.Time
			var currentEventTimeMs int64

			switch evt := evt.(type) {
			case *apievents.SessionStart:
				sessionStartTime = evt.Time
				lastEventTime = evt.Time
				lastEventTimeMs = 0

				parts := strings.Split(evt.TerminalSize, ":")

        if len(parts) == 2 {
          width, wErr := strconv.Atoi(parts[0])
          height, hErr := strconv.Atoi(parts[1])

          if cmp.Or(wErr, hErr) == nil {
            startRows = height
            startCols = width

            vt.Resize(width, height)
          }
        }

			case *apievents.SessionPrint:
				currentEventTime = evt.Time
				currentEventTimeMs = int64(currentEventTime.Sub(sessionStartTime) / time.Millisecond)

				if lastEventTime != (time.Time{}) && currentEventTime.Sub(lastEventTime).Milliseconds() > inactivityThreshold {
					inactivityStart := lastEventTimeMs
					inactivityEnd := currentEventTimeMs
					userEvents = append(userEvents, SessionEvent{
						Type:      "inactivity",
						StartTime: inactivityStart,
						EndTime:   inactivityEnd,
					})
				}

				// Track statistics
				eventCount++
				totalDataSize += len(evt.Data)

				// For high-volume sessions, limit data processing
				if len(dataBuffer) < maxDataPerThumbnail {
					dataBuffer = append(dataBuffer, evt.Data...)
				} else {
					// Skip data that would exceed our limit
					skippedDataSize += len(evt.Data)
				}

				if currentEventTimeMs >= nextThumbnailTime {
					// Write buffered data at once before serializing
					if len(dataBuffer) > 0 {
						vt.Write(dataBuffer)
						dataBuffer = dataBuffer[:0] // Reset buffer
					}

					serialized := terminal.Serialize(vt)

					thumbnails = append(thumbnails, Thumbnail{
						Screen:    *serialized,
						StartTime: currentEventTimeMs,
						EndTime:   currentEventTimeMs + intervalMs - 1,
					})

					nextThumbnailTime = currentEventTimeMs + intervalMs
				}

				lastEventTime = currentEventTime
				lastEventTimeMs = currentEventTimeMs

			case *apievents.SessionCommand:
				fmt.Printf("Command event: %v\n", evt)

			case *apievents.SessionJoin:
				activeUsers[evt.User] = int64(evt.Time.Sub(sessionStartTime) / time.Millisecond)

			case *apievents.SessionLeave:
				if startTime, ok := activeUsers[evt.User]; ok {
					userEvents = append(userEvents, SessionEvent{
						Type:      "join",
						User:      evt.User,
						StartTime: startTime,
						EndTime:   int64(evt.Time.Sub(sessionStartTime) / time.Millisecond),
					})
					delete(activeUsers, evt.User)
				}

			case *apievents.SessionEnd:
				currentEventTime = evt.EndTime
				currentEventTimeMs = int64(currentEventTime.Sub(sessionStartTime) / time.Millisecond)

				if lastEventTime != (time.Time{}) && currentEventTime.Sub(lastEventTime).Milliseconds() > inactivityThreshold {
					inactivityStart := lastEventTimeMs
					inactivityEnd := currentEventTimeMs
					userEvents = append(userEvents, SessionEvent{
						Type:      "inactivity",
						StartTime: inactivityStart,
						EndTime:   inactivityEnd,
					})
				}

				endTime := int64(evt.EndTime.Sub(evt.StartTime) / time.Millisecond)

				// Flush any remaining buffered data before final serialization
				if len(dataBuffer) > 0 {
					vt.Write(dataBuffer)
					dataBuffer = nil
				}

				serialized := terminal.Serialize(vt)

				thumbnails = append(thumbnails, Thumbnail{
					Screen:    *serialized,
					StartTime: endTime,
					EndTime:   endTime,
				})

				for user, startTime := range activeUsers {
					userEvents = append(userEvents, SessionEvent{
						Type:      "join",
						User:      user,
						StartTime: startTime,
						EndTime:   endTime,
					})
				}

				fmt.Printf("sessionDetails: processed %d events, %d MB total data, %d MB skipped, %d thumbnails\n", 
					eventCount, totalDataSize/(1024*1024), skippedDataSize/(1024*1024), len(thumbnails))

				return SessionEventsResponse{
					Duration:   int64(evt.EndTime.Sub(evt.StartTime) / time.Millisecond),
					Thumbnails: thumbnails,
					Events:     userEvents,
					StartCols:  startCols,
					StartRows:  startRows,
				}, nil

			case *apievents.Resize:
				parts := strings.Split(evt.TerminalSize, ":")

				if len(parts) == 2 {
					width, wErr := strconv.Atoi(parts[0])
					height, hErr := strconv.Atoi(parts[1])

					if cmp.Or(wErr, hErr) == nil {
						userEvents = append(userEvents, SessionEvent{
							Type:      "resize",
							StartTime: int64(evt.Time.Sub(sessionStartTime) / time.Millisecond),
							Cols:      width,
							Rows:      height,
						})

						// Flush buffered data before resizing to ensure terminal state is current
						if len(dataBuffer) > 0 {
							vt.Write(dataBuffer)
							dataBuffer = dataBuffer[:0]
						}

						vt.Resize(width, height)
					}
				}
			}
		}
	}
}

func (h *Handler) sessionFrame(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sctx *SessionContext,
	site reversetunnelclient.RemoteSite,
) (interface{}, error) {
	sID := p.ByName("sid")
	if sID == "" {
		return nil, trace.BadParameter("missing session ID in request URL")
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	type FrameResponse struct {
		Screen    terminal.SerializedTerminal `json:"screen"`
		Timestamp int64                       `json:"timestamp"`
		Cols      int                         `json:"cols"`
		Rows      int                         `json:"rows"`
	}

	const targetTimeMs int64 = 15000

	vt := vt10x.New()
	var sessionStartTime time.Time
	var cols, rows int

	evts, errs := clt.StreamSessionEvents(metadata.WithSessionRecordingFormatContext(ctx, teleport.PTY), session.ID(sID), 0)
	for {
		select {
		case err := <-errs:
			return nil, trace.Wrap(err)
		case evt, ok := <-evts:
			if !ok {
				return nil, trace.NotFound("could not find end event for session %v", sID)
			}

			switch evt := evt.(type) {
			case *apievents.SessionStart:
				sessionStartTime = evt.Time

				parts := strings.Split(evt.TerminalSize, ":")
				if len(parts) == 2 {
					width, wErr := strconv.Atoi(parts[0])
					height, hErr := strconv.Atoi(parts[1])

					if cmp.Or(wErr, hErr) == nil {
						cols = width
						rows = height
						vt.Resize(width, height)
					}
				}

			case *apievents.SessionPrint:
				currentTimeMs := int64(evt.Time.Sub(sessionStartTime) / time.Millisecond)
				vt.Write(evt.Data)

				if currentTimeMs >= targetTimeMs {
					serialized := terminal.Serialize(vt)
					return FrameResponse{
						Screen:    *serialized,
						Timestamp: currentTimeMs,
						Cols:      cols,
						Rows:      rows,
					}, nil
				}

			case *apievents.Resize:
				parts := strings.Split(evt.TerminalSize, ":")
				if len(parts) == 2 {
					width, wErr := strconv.Atoi(parts[0])
					height, hErr := strconv.Atoi(parts[1])

					if cmp.Or(wErr, hErr) == nil {
						cols = width
						rows = height
						vt.Resize(width, height)
					}
				}

			case *apievents.SessionEnd:
				endTimeMs := int64(evt.EndTime.Sub(sessionStartTime) / time.Millisecond)
				serialized := terminal.Serialize(vt)
				return FrameResponse{
					Screen:    *serialized,
					Timestamp: endTimeMs,
					Cols:      cols,
					Rows:      rows,
				}, nil
			}
		}
	}
}

func (h *Handler) ttyPlaybackHandle(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sctx *SessionContext,
	site reversetunnelclient.RemoteSite,
) (any, error) {
	sID := p.ByName("sid")
	if sID == "" {
		return nil, trace.BadParameter("missing session ID in request URL")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	h.logger.DebugContext(r.Context(), "upgrading to websocket")
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.WarnContext(r.Context(), "failed upgrade", "error", err)
		// if Upgrade fails, it automatically replies with an HTTP error
		// (this means we don't need to return an error here)
		return nil, nil
	}

	player, err := player.New(&player.Config{
		Clock:     h.clock,
		Log:       h.logger,
		SessionID: session.ID(sID),
		Streamer:  clt,
		Context:   r.Context(),
	})
	if err != nil {
		h.logger.WarnContext(r.Context(), "player error", "error", err)
		writeError(ws, err)
		return nil, nil
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		defer cancel()
		for {
			typ, b, err := ws.ReadMessage()
			if err != nil {
				if !utils.IsOKNetworkError(err) {
					h.logger.WarnContext(ctx, "websocket read error", "error", err)
				}
				return
			}

			if typ != websocket.BinaryMessage {
				h.logger.DebugContext(ctx, "skipping unknown websocket message type", "message_type", logutils.TypeAttr(typ))
				continue
			}

			if err := handlePlaybackAction(b, player); err != nil {
				h.logger.WarnContext(ctx, "skipping bad action", "error", err)
				continue
			}
		}
	}()

	go func() {
		defer cancel()
		defer func() {
			h.logger.DebugContext(ctx, "closing websocket")
			if err := ws.WriteMessage(websocket.CloseMessage, nil); err != nil {
				h.logger.DebugContext(r.Context(), "error sending close message", "error", err)
			}
			if err := ws.Close(); err != nil {
				h.logger.DebugContext(ctx, "error closing websocket", "error", err)
			}
		}()

		player.Play()
		defer player.Close()

		headerBuf := make([]byte, 11)
		headerBuf[0] = messageTypePTY

		writePTY := func(b []byte, delay uint64) error {
			writer, err := ws.NextWriter(websocket.BinaryMessage)
			if err != nil {
				return trace.Wrap(err, "getting websocket writer")
			}
			msgLen := uint16(len(b) + 8)
			binary.BigEndian.PutUint16(headerBuf[1:], msgLen)
			binary.BigEndian.PutUint64(headerBuf[3:], delay)
			if _, err := writer.Write(headerBuf); err != nil {
				return trace.Wrap(err, "writing message header")
			}

			// TODO(zmb3): consider optimizing this by bufering for very large sessions
			// (wait up to N ms to batch events into a single websocket write).
			if _, err := writer.Write(b); err != nil {
				return trace.Wrap(err, "writing PTY data")
			}

			if err := writer.Close(); err != nil {
				return trace.Wrap(err, "closing websocket writer")
			}

			return nil
		}

		writeSize := func(size string) error {
			ts, err := session.UnmarshalTerminalParams(size)
			if err != nil {
				h.logger.DebugContext(ctx, "Ignoring invalid terminal size", "terminal_size", size)
				return nil // don't abort playback due to a bad event
			}

			msg := make([]byte, 7)
			msg[0] = messageTypeResize
			binary.BigEndian.PutUint16(msg[1:], 4)
			binary.BigEndian.PutUint16(msg[3:], uint16(ts.W))
			binary.BigEndian.PutUint16(msg[5:], uint16(ts.H))

			return trace.Wrap(ws.WriteMessage(websocket.BinaryMessage, msg))
		}

		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-player.C():
				if !ok {
					// send any playback errors to the browser
					if err := writeError(ws, player.Err()); err != nil {
						h.logger.WarnContext(ctx, "failed to send error message to browser", "error", err)
					}
					return
				}

				switch evt := evt.(type) {
				case *apievents.SessionStart:
					if err := writeSize(evt.TerminalSize); err != nil {
						h.logger.DebugContext(ctx, "Failed to write resize", "error", err)
						return
					}

				case *apievents.SessionPrint:
					if err := writePTY(evt.Data, uint64(evt.DelayMilliseconds)); err != nil {
						h.logger.DebugContext(ctx, "Failed to send PTY data", "error", err)
						return
					}

				case *apievents.SessionEnd:
					// send empty PTY data - this will ensure that any dead time
					// at the end of the recording is processed and the allow
					// the progress bar to go to 100%
					if err := writePTY(nil, uint64(evt.EndTime.Sub(evt.StartTime)/time.Millisecond)); err != nil {
						h.logger.DebugContext(ctx, "Failed to send session end data", "error", err)
						return
					}

				case *apievents.Resize:
					if err := writeSize(evt.TerminalSize); err != nil {
						h.logger.DebugContext(ctx, "Failed to write resize", "error", err)
						return
					}

				case *apievents.SessionLeave: // do nothing

				default:
					h.logger.DebugContext(ctx, "unexpected event type", "event_type", logutils.TypeAttr(evt))
				}
			}
		}
	}()

	<-ctx.Done()
	return nil, nil
}

func (h *Handler) sessionEventsWs(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sctx *SessionContext,
	site reversetunnelclient.RemoteSite,
) (interface{}, error) {
	sID := p.ByName("sid")
	if sID == "" {
		return nil, trace.BadParameter("missing session ID in request URL")
	}

	h.logger.DebugContext(r.Context(), "upgrading to websocket")
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.WarnContext(r.Context(), "failed upgrade", "error", err)
		return nil, nil
	}

	ctx := r.Context()
	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		writeError(ws, err)
		return nil, nil
	}

	handler := ttyplayback.NewSessionEventsHandler(ctx, ws, clt, sID, h.logger)
	handler.Run()

	return nil, nil
}

func writeError(ws *websocket.Conn, err error) error {
	if err == nil {
		return nil
	}

	b := new(bytes.Buffer)
	b.WriteByte(messageTypeError)

	msg := trace.UserMessage(err)
	l := 1 /* severity */ + 2 /* msg length */ + len(msg)
	binary.Write(b, binary.BigEndian, uint16(l))
	b.WriteByte(severityError)
	binary.Write(b, binary.BigEndian, uint16(len(msg)))
	b.WriteString(msg)

	return trace.Wrap(ws.WriteMessage(websocket.BinaryMessage, b.Bytes()))
}

type play interface {
	Play() error
	Pause() error
	SetPos(time.Duration) error
}

// handlePlaybackAction processes a playback message
// received from the browser
func handlePlaybackAction(b []byte, p play) error {
	if len(b) < 3 {
		return trace.BadParameter("invalid playback message")
	}

	msgType := b[0]
	msgLen := binary.BigEndian.Uint16(b[1:])

	if len(b) < int(msgLen)+3 {
		return trace.BadParameter("invalid message length")
	}

	payload := b[3:]
	payload = payload[:msgLen]

	switch msgType {
	case messageTypePlayPause:
		if len(payload) != 1 {
			return trace.BadParameter("invalid play/pause command")
		}
		switch action := payload[0]; action {
		case actionPlay:
			p.Play()
		case actionPause:
			p.Pause()
		default:
			return trace.BadParameter("invalid play/pause action %v", action)
		}
	case messageTypeSeek:
		if len(payload) != 8 {
			return trace.BadParameter("invalid seek message")
		}
		pos := binary.BigEndian.Uint64(payload)
		p.SetPos(time.Duration(pos) * time.Millisecond)
	}

	return nil
}

/*

# Websocket Protocol for TTY Playback:

During playback, the Teleport proxy sends session data to the browser
and the browser sends playback commands (play/pause, seek, etc) to the
proxy.

Each message conforms to the following binary protocol.

## Message Header

The message header starts with a 1-byte identifier followed by a 2-byte
(big endian) integer containing the number of bytes following the header.
This length field does not include the 3-byte header.

## Messages

### 1 - PTY data

This message is used to send recorded PTY data to the browser.

- Message ID: 1
- 8-byte timestamp (milliseconds since session start)
- PTY data

### 2 - Error

This message is used to indicate that an error has occurred.

- Message ID: 2
- 1 byte severity (1=error)
- 2-byte error message length
- variable length error message (UTF-8 text)

### 3 - Play/Pause

This message is sent from the browser to the server to pause
or resume playback.

- Message ID: 3
- 1-byte code (0=play, 1=pause)

### 4 - Seek

This message is used to seek to a new position in the recording.

- Message ID: 4
- 8-byte timestamp (milliseconds since session start)

### 5 - Resize

This message is used to indicate that the terminal was resized.

- Message ID: 5
- 2-byte width
- 2-byte height

*/
