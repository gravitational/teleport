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
	"context"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/hinshun/vt10x"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/metadata"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/player"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/web/ttyplayback"
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
		Screen    ttyplayback.SerializedTerminal `json:"screen"`
		StartTime int64                          `json:"start_time"`
		EndTime   int64                          `json:"end_time"`
	}

	type SessionEventsResponse struct {
		Duration   int64       `json:"duration"`
		Thumbnails []Thumbnail `json:"thumbnails"`
	}

	var sessionEvents []ttyplayback.Event

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
			case *apievents.SessionPrint:
				sessionEvents = append(sessionEvents, ttyplayback.Event{
					Type: "print",
					Data: string(evt.Data),
					Time: evt.DelayMilliseconds,
				})
			case *apievents.SessionEnd:
				sessionEvents = append(sessionEvents, ttyplayback.Event{
					Type: "end",
					Data: evt.EndTime.Format(time.RFC3339),
					Time: int64(evt.EndTime.Sub(evt.StartTime) / time.Millisecond),
				})

				type FrameAtInterval struct {
					frame ttyplayback.Frame
					start int64
					end   int64
				}

				frameAtInterval := make([]FrameAtInterval, 0)
				currentFrameIdx := 0

				intervalMs := 500
				maxTime := sessionEvents[len(sessionEvents)-1].Time

				iter := ttyplayback.NewSliceIterator(sessionEvents)
				framesIter := ttyplayback.Frames(iter)

				frames := framesIter.CollectAll()

				for step := 0; step <= int(maxTime)/intervalMs; step++ {
					targetTime := int64(step * intervalMs)

					for currentFrameIdx < len(frames)-1 && frames[currentFrameIdx+1].Time <= targetTime {
						currentFrameIdx++
					}

					frameAtInterval = append(frameAtInterval, FrameAtInterval{
						frame: frames[currentFrameIdx],
						start: targetTime,
						end:   targetTime + int64(intervalMs) - 1,
					})
				}

				thumbnails := make([]Thumbnail, 0, len(frameAtInterval))
				for _, frame := range frameAtInterval {
					thumbnails = append(thumbnails, Thumbnail{
						Screen: ttyplayback.SerializedTerminal{
							Data:    frame.frame.Ansi,
							Cols:    frame.frame.Cols,
							Rows:    frame.frame.Rows,
							CursorX: frame.frame.Cursor.X,
							CursorY: frame.frame.Cursor.Y,
						},
						StartTime: frame.start,
						EndTime:   frame.end,
					})
				}

				return SessionEventsResponse{
					Duration:   int64(evt.EndTime.Sub(evt.StartTime) / time.Millisecond),
					Thumbnails: thumbnails,
				}, nil
			case *apievents.Resize:
				sessionEvents = append(sessionEvents, ttyplayback.Event{
					Type: "resize",
					Data: evt.TerminalSize,
				})
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

	var startFromTime int64

	query := r.URL.Query()
	// limit value is expected to be non empty and convertible to int.
	startTimeValue := query.Get("start_time")
	if startTimeValue == "" {
		startFromTime = 0
	} else {
		var err error
		startFromTime, err = strconv.ParseInt(startTimeValue, 10, 64)
		if err != nil {
			return nil, trace.BadParameter("invalid start index: %v", err)
		}
		if startFromTime < 0 {
			return nil, trace.BadParameter("start index must be non-negative")
		}
	}

	var limit int64 = 500
	limitValue := query.Get("limit")
	if limitValue != "" {
		var err error
		limit, err = strconv.ParseInt(limitValue, 10, 64)
		if err != nil {
			return nil, trace.BadParameter("invalid limit: %v", err)
		}
		if limit <= 0 {
			return nil, trace.BadParameter("limit must be a positive integer")
		}
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	type SessionEventsResponse struct {
		CurrentScreen ttyplayback.SerializedTerminal `json:"currentScreen"`
		Duration      int64                          `json:"duration"`
		Events        []ttyplayback.Event            `json:"events"`
	}

	var currentScreen ttyplayback.SerializedTerminal

	var sessionEvents []ttyplayback.Event

	var lastEvent *apievents.SessionPrint

	vt := vt10x.New()

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
				if startFromTime > 0 {
					// If the event is before the start time, skip it.
					continue
				}
				sessionEvents = append(sessionEvents, ttyplayback.Event{
					Type: "start",
					Data: evt.TerminalSize,
				})
			case *apievents.SessionPrint:
				if evt.DelayMilliseconds < startFromTime {
					if _, err := vt.Write(evt.Data); err != nil {
						h.logger.WarnContext(ctx, "failed to write to terminal", "error", err, "data", evt.Data)

						continue
					}

					lastEvent = evt

					continue
				}

				if lastEvent != nil {
					currentScreen = ttyplayback.SerializeTerminal(vt)

					lastEvent = nil
				}

				sessionEvents = append(sessionEvents, ttyplayback.Event{
					Type: "print",
					Data: string(evt.Data),
					Time: evt.DelayMilliseconds,
				})

				if int64(len(sessionEvents)) >= limit {
					// If we have reached the limit, return the events collected so far.
					return SessionEventsResponse{
						CurrentScreen: currentScreen,
						Duration:      int64(evt.DelayMilliseconds),
						Events:        sessionEvents,
					}, nil
				}
			case *apievents.SessionEnd:
				sessionEvents = append(sessionEvents, ttyplayback.Event{
					Type: "end",
					Data: evt.EndTime.Format(time.RFC3339),
					Time: int64(evt.EndTime.Sub(evt.StartTime) / time.Millisecond),
				})
				return SessionEventsResponse{
					CurrentScreen: currentScreen,
					Duration:      int64(evt.EndTime.Sub(evt.StartTime) / time.Millisecond),
					Events:        sessionEvents,
				}, nil
			case *apievents.Resize:
				parts := strings.Split(evt.TerminalSize, ":")
				if len(parts) != 2 {
					h.logger.WarnContext(ctx, "invalid terminal size format", "size", evt.TerminalSize)
					continue
				}

				width, err := strconv.Atoi(parts[0])
				if err != nil {
					h.logger.WarnContext(ctx, "invalid terminal width", "width", parts[0], "error", err)
					continue
				}

				height, err := strconv.Atoi(parts[1])
				if err != nil {
					h.logger.WarnContext(ctx, "invalid terminal height", "height", parts[1], "error", err)
					continue
				}

				vt.Resize(width, height)

				if startFromTime > 0 {
					// If the resize event is before the start time, skip it.
					continue
				}

				sessionEvents = append(sessionEvents, ttyplayback.Event{
					Type: "resize",
					Data: evt.TerminalSize,
				})
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

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		writeError(ws, err)
		return nil, nil
	}

	type sessionEventStreamer struct {
		events            <-chan apievents.AuditEvent
		errors            <-chan error
		cancel            context.CancelFunc
		vt                vt10x.Terminal
		vtMutex           sync.RWMutex
		lastProcessedTime int64
		currentTime       int64
		terminalSize      string
		pendingEvent      apievents.AuditEvent
	}

	var streamer *sessionEventStreamer
	var streamerMutex sync.Mutex

	rebuildVTState := func(upToTime int64) (*vt10x.Terminal, error) {
		streamCtx, streamCancel := context.WithCancel(ctx)
		defer streamCancel()

		vt := vt10x.New()
		evts, errs := clt.StreamSessionEvents(
			metadata.WithSessionRecordingFormatContext(streamCtx, teleport.PTY),
			session.ID(sID),
			0,
		)

		for {
			select {
			case <-streamCtx.Done():
				return &vt, nil
			case err := <-errs:
				if err != nil {
					return nil, err
				}
				return &vt, nil
			case evt, ok := <-evts:
				if !ok {
					return &vt, nil
				}

				switch evt := evt.(type) {
				case *apievents.SessionStart:
					parts := strings.Split(evt.TerminalSize, ":")
					if len(parts) == 2 {
						width, _ := strconv.Atoi(parts[0])
						height, _ := strconv.Atoi(parts[1])
						vt.Resize(width, height)
					}

				case *apievents.SessionPrint:
					if evt.DelayMilliseconds > upToTime {
						return &vt, nil
					}
					if _, err := vt.Write(evt.Data); err != nil {
						h.logger.WarnContext(streamCtx, "failed to write to terminal", "error", err)
					}

				case *apievents.Resize:
					parts := strings.Split(evt.TerminalSize, ":")
					if len(parts) == 2 {
						width, err := strconv.Atoi(parts[0])
						if err == nil {
							height, err := strconv.Atoi(parts[1])
							if err == nil {
								vt.Resize(width, height)
							}
						}
					}

				case *apievents.SessionEnd:
					return &vt, nil
				}
			}
		}
	}

	initStreamer := func(startTime int64) error {
		streamerMutex.Lock()
		defer streamerMutex.Unlock()

		if streamer != nil && streamer.cancel != nil {
			streamer.cancel()
		}

		streamCtx, streamCancel := context.WithCancel(ctx)

		vt := vt10x.New()
		if startTime > 0 {
			rebuiltVT, err := rebuildVTState(startTime)
			if err != nil {
				return err
			}
			if rebuiltVT != nil {
				vt = *rebuiltVT
			}
		}

		evts, errs := clt.StreamSessionEvents(
			metadata.WithSessionRecordingFormatContext(streamCtx, teleport.PTY),
			session.ID(sID),
			startTime,
		)

		streamer = &sessionEventStreamer{
			events:            evts,
			errors:            errs,
			cancel:            streamCancel,
			vt:                vt,
			lastProcessedTime: startTime,
			currentTime:       startTime,
		}

		return nil
	}

	type SessionEventsRequest struct {
		Type                 string `json:"type"`
		StartTime            int64  `json:"startTime"`
		EndTime              int64  `json:"endTime"`
		RequestCurrentScreen bool   `json:"requestCurrentScreen"`
	}

	type SessionEventsResponse struct {
		Type          string                         `json:"type"`
		CurrentScreen ttyplayback.SerializedTerminal `json:"currentScreen,omitempty"`
		Duration      int64                          `json:"duration"`
		Events        []ttyplayback.Event            `json:"events"`
		HasMore       bool                           `json:"hasMore"`
	}

	go func() {
		defer cancel()
		for {
			_, b, err := ws.ReadMessage()
			if err != nil {
				if !utils.IsOKNetworkError(err) {
					h.logger.WarnContext(ctx, "websocket read error", "error", err)
				}
				return
			}

			var req SessionEventsRequest
			if err := json.Unmarshal(b, &req); err != nil {
				h.logger.WarnContext(ctx, "failed to unmarshal request", "error", err)
				continue
			}

			switch req.Type {
			case "fetch":
				needsReinit := false
				streamerMutex.Lock()
				if streamer == nil {
					needsReinit = true
				} else if req.StartTime < streamer.lastProcessedTime {
					needsReinit = true
				}
				streamerMutex.Unlock()

				if needsReinit {
					if err := initStreamer(req.StartTime); err != nil {
						writeError(ws, err)
						continue
					}
				}

				go func() {
					sessionEvents := make([]ttyplayback.Event, 0)
					var currentScreen ttyplayback.SerializedTerminal
					var duration int64
					hasMore := true

					streamerMutex.Lock()
					localStreamer := streamer
					streamerMutex.Unlock()

					if localStreamer == nil {
						writeError(ws, trace.BadParameter("streamer not initialized"))
						return
					}

					if localStreamer.pendingEvent != nil {
						evt := localStreamer.pendingEvent
						localStreamer.pendingEvent = nil

						switch evt := evt.(type) {
						case *apievents.SessionPrint:
							if evt.DelayMilliseconds >= req.StartTime && evt.DelayMilliseconds <= req.EndTime {
								sessionEvents = append(sessionEvents, ttyplayback.Event{
									Type: "print",
									Data: string(evt.Data),
									Time: evt.DelayMilliseconds,
								})
								duration = evt.DelayMilliseconds
							}

							localStreamer.vtMutex.Lock()
							if _, err := localStreamer.vt.Write(evt.Data); err != nil {
								h.logger.WarnContext(ctx, "failed to write to terminal", "error", err)
							}
							localStreamer.currentTime = evt.DelayMilliseconds
							localStreamer.vtMutex.Unlock()

							if evt.DelayMilliseconds > req.EndTime {
								localStreamer.pendingEvent = evt
								goto sendResponse
							}
						}
					}

				processLoop:
					for {
						select {
						case <-ctx.Done():
							return
						case err := <-localStreamer.errors:
							if err != nil {
								writeError(ws, err)
								return
							}
							hasMore = false
							break processLoop
						case evt, ok := <-localStreamer.events:
							if !ok {
								hasMore = false
								break processLoop
							}

							switch evt := evt.(type) {
							case *apievents.SessionStart:
								if req.StartTime <= 0 {
									sessionEvents = append(sessionEvents, ttyplayback.Event{
										Type: "start",
										Data: evt.TerminalSize,
									})
								}
								localStreamer.terminalSize = evt.TerminalSize
								parts := strings.Split(evt.TerminalSize, ":")
								if len(parts) == 2 {
									width, _ := strconv.Atoi(parts[0])
									height, _ := strconv.Atoi(parts[1])
									localStreamer.vtMutex.Lock()
									localStreamer.vt.Resize(width, height)
									localStreamer.vtMutex.Unlock()
								}

							case *apievents.SessionPrint:
								localStreamer.vtMutex.Lock()
								if _, err := localStreamer.vt.Write(evt.Data); err != nil {
									h.logger.WarnContext(ctx, "failed to write to terminal", "error", err)
									localStreamer.vtMutex.Unlock()
									continue
								}
								localStreamer.currentTime = evt.DelayMilliseconds
								localStreamer.vtMutex.Unlock()

								if evt.DelayMilliseconds >= req.StartTime && evt.DelayMilliseconds <= req.EndTime {
									sessionEvents = append(sessionEvents, ttyplayback.Event{
										Type: "print",
										Data: string(evt.Data),
										Time: evt.DelayMilliseconds,
									})
									duration = evt.DelayMilliseconds
								}

								if evt.DelayMilliseconds > req.EndTime {
									localStreamer.pendingEvent = evt
									break processLoop
								}

								localStreamer.lastProcessedTime = evt.DelayMilliseconds

							case *apievents.SessionEnd:
								endTime := int64(evt.EndTime.Sub(evt.StartTime) / time.Millisecond)
								if endTime >= req.StartTime && endTime <= req.EndTime {
									sessionEvents = append(sessionEvents, ttyplayback.Event{
										Type: "end",
										Data: evt.EndTime.Format(time.RFC3339),
										Time: endTime,
									})
									duration = endTime
								}
								hasMore = false
								break processLoop

							case *apievents.Resize:
								parts := strings.Split(evt.TerminalSize, ":")
								if len(parts) == 2 {
									width, err := strconv.Atoi(parts[0])
									if err == nil {
										height, err := strconv.Atoi(parts[1])
										if err == nil {
											localStreamer.vtMutex.Lock()
											localStreamer.vt.Resize(width, height)
											localStreamer.vtMutex.Unlock()

											sessionEvents = append(sessionEvents, ttyplayback.Event{
												Type: "resize",
												Data: evt.TerminalSize,
											})
										}
									}
								}
							}
						}
					}

				sendResponse:
					localStreamer.vtMutex.RLock()
					currentScreen = ttyplayback.SerializeTerminal(localStreamer.vt)
					localStreamer.vtMutex.RUnlock()

					resp := SessionEventsResponse{
						Type:     "events",
						Duration: duration,
						Events:   sessionEvents,
						HasMore:  hasMore,
					}

					if req.RequestCurrentScreen {
						resp.CurrentScreen = currentScreen
					}

					respData, err := json.Marshal(resp)
					if err != nil {
						h.logger.WarnContext(ctx, "failed to marshal response", "error", err)
						return
					}

					if err := ws.WriteMessage(websocket.TextMessage, respData); err != nil {
						h.logger.WarnContext(ctx, "failed to write response", "error", err)
						return
					}
				}()

			case "close":
				return

			default:
				h.logger.WarnContext(ctx, "unknown request type", "type", req.Type)
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
			streamerMutex.Lock()
			if streamer != nil && streamer.cancel != nil {
				streamer.cancel()
			}
			streamerMutex.Unlock()
		}()

		<-ctx.Done()
	}()

	<-ctx.Done()
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
