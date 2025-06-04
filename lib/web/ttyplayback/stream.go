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
	"github.com/hinshun/vt10x"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/metadata"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/ttyplayback/terminal"
)

const (
	// Request types
	RequestTypeFetch byte = 1
	RequestTypeClose byte = 2

	// Event types
	EventTypeStart        byte = 1
	EventTypeStop         byte = 2
	EventTypeError        byte = 3
	EventTypeSessionStart byte = 4
	EventTypeSessionPrint byte = 5
	EventTypeSessionEnd   byte = 6
	EventTypeResize       byte = 7
	EventTypeScreen       byte = 8
)

type BinaryRequest struct {
	Type                 byte
	StartTime            int64
	EndTime              int64
	RequestCurrentScreen bool
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
}

type SessionEventsHandler struct {
	ctx           context.Context
	cancel        context.CancelFunc
	ws            *websocket.Conn
	clt           authclient.ClientI
	sessionID     string
	logger        *slog.Logger
	streamer      *sessionEventStreamer
	streamerMutex sync.Mutex
	writeMutex    sync.Mutex
	activeStream  context.CancelFunc
	streamMutex   sync.Mutex
}

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

func (s *SessionEventsHandler) Run() {
	defer s.cancel()

	go s.readLoop()
	go s.cleanupLoop()

	<-s.ctx.Done()
}

func (s *SessionEventsHandler) readLoop() {
	defer s.cancel()
	for {
		messageType, b, err := s.ws.ReadMessage()
		if err != nil {
			if !utils.IsOKNetworkError(err) {
				s.logger.WarnContext(s.ctx, "websocket read error", "error", err)
			}
			return
		}

		if messageType != websocket.BinaryMessage {
			s.logger.WarnContext(s.ctx, "expected binary message", "type", messageType)
			continue
		}

		req, err := s.decodeBinaryRequest(b)
		if err != nil {
			s.logger.WarnContext(s.ctx, "failed to decode request", "error", err)
			continue
		}

		switch req.Type {
		case RequestTypeFetch:
			s.handleFetchRequest(req)
		case RequestTypeClose:
			return
		default:
			s.logger.WarnContext(s.ctx, "unknown request type", "type", req.Type)
		}
	}
}

func (s *SessionEventsHandler) decodeBinaryRequest(data []byte) (*BinaryRequest, error) {
	if len(data) < 17 {
		return nil, fmt.Errorf("request too short")
	}

	req := &BinaryRequest{
		Type: data[0],
	}

	req.StartTime = int64(binary.BigEndian.Uint64(data[1:9]))
	req.EndTime = int64(binary.BigEndian.Uint64(data[9:17]))

	if len(data) > 17 {
		req.RequestCurrentScreen = data[17] == 1
	}

	return req, nil
}

func (s *SessionEventsHandler) writeBinaryEvent(eventType byte, timestamp int64, data []byte) error {
	s.writeMutex.Lock()
	defer s.writeMutex.Unlock()

	buf := make([]byte, 13+len(data))
	buf[0] = eventType
	binary.BigEndian.PutUint64(buf[1:9], uint64(timestamp))
	binary.BigEndian.PutUint32(buf[9:13], uint32(len(data)))
	if len(data) > 0 {
		copy(buf[13:], data)
	}

	return s.ws.WriteMessage(websocket.BinaryMessage, buf)
}

func (s *SessionEventsHandler) cleanupLoop() {
	defer s.cancel()
	defer func() {
		s.logger.DebugContext(s.ctx, "closing websocket")
		if err := s.ws.WriteMessage(websocket.CloseMessage, nil); err != nil {
			s.logger.DebugContext(s.ctx, "error sending close message", "error", err)
		}
		if err := s.ws.Close(); err != nil {
			s.logger.DebugContext(s.ctx, "error closing websocket", "error", err)
		}
		s.streamerMutex.Lock()
		if s.streamer != nil && s.streamer.cancel != nil {
			s.streamer.cancel()
		}
		s.streamerMutex.Unlock()
	}()

	<-s.ctx.Done()
}

func (s *SessionEventsHandler) handleFetchRequest(req *BinaryRequest) {
	s.streamMutex.Lock()
	if s.activeStream != nil {
		s.activeStream()
	}
	streamCtx, streamCancel := context.WithCancel(s.ctx)
	s.activeStream = streamCancel
	s.streamMutex.Unlock()

	needsReinit := false
	s.streamerMutex.Lock()
	if s.streamer == nil {
		needsReinit = true
	} else if req.StartTime < s.streamer.lastProcessedTime {
		needsReinit = true
	}
	s.streamerMutex.Unlock()

	if needsReinit {
		if err := s.initStreamer(req.StartTime); err != nil {
			s.writeBinaryEvent(EventTypeError, 0, []byte(err.Error()))
			return
		}
	}

	go s.streamEvents(streamCtx, req)
}

func (s *SessionEventsHandler) streamEvents(streamCtx context.Context, req *BinaryRequest) {
	defer func() {
		s.streamMutex.Lock()
		if s.activeStream != nil {
			s.activeStream()
			s.activeStream = nil
		}
		s.streamMutex.Unlock()
	}()

	if err := s.writeBinaryEvent(EventTypeStart, req.StartTime, nil); err != nil {
		s.logger.WarnContext(s.ctx, "failed to send start event", "error", err)
		return
	}

	s.streamerMutex.Lock()
	localStreamer := s.streamer
	s.streamerMutex.Unlock()

	if localStreamer == nil {
		s.writeBinaryEvent(EventTypeError, 0, []byte("streamer not initialized"))
		return
	}

	if req.RequestCurrentScreen {
		localStreamer.vtMutex.RLock()
		screen := terminal.Serialize(localStreamer.vt)
		localStreamer.vtMutex.RUnlock()

		screenData := encodeSerializedTerminal(screen)
		if err := s.writeBinaryEvent(EventTypeScreen, localStreamer.currentTime, screenData); err != nil {
			s.logger.WarnContext(s.ctx, "failed to send screen", "error", err)
		}
	}

	for {
		select {
		case <-streamCtx.Done():
			s.writeBinaryEvent(EventTypeStop, 0, nil)
			return
		case <-s.ctx.Done():
			return
		case err := <-localStreamer.errors:
			if err != nil {
				s.writeBinaryEvent(EventTypeError, 0, []byte(err.Error()))
			}
			s.writeBinaryEvent(EventTypeStop, 0, nil)
			return
		case evt, ok := <-localStreamer.events:
			if !ok {
				s.writeBinaryEvent(EventTypeStop, 0, nil)
				return
			}

			if !s.processAndSendEvent(evt, req, localStreamer) {
				s.writeBinaryEvent(EventTypeStop, 0, nil)
				return
			}
		}
	}
}

func (s *SessionEventsHandler) processAndSendEvent(evt apievents.AuditEvent, req *BinaryRequest, streamer *sessionEventStreamer) bool {
	switch evt := evt.(type) {
	case *apievents.SessionStart:
		streamer.terminalSize = evt.TerminalSize
		parts := strings.Split(evt.TerminalSize, ":")
		if len(parts) == 2 {
			width, wErr := strconv.Atoi(parts[0])
			height, hErr := strconv.Atoi(parts[1])
			if err := cmp.Or(wErr, hErr); err != nil {
				s.logger.WarnContext(s.ctx, "failed to parse terminal size", "error", err)
				return false
			}
			streamer.vtMutex.Lock()
			streamer.vt.Resize(width, height)
			streamer.vtMutex.Unlock()
		}

		if req.StartTime <= 0 {
			if err := s.writeBinaryEvent(EventTypeSessionStart, 0, []byte(evt.TerminalSize)); err != nil {
				s.logger.WarnContext(s.ctx, "failed to send session start", "error", err)
			}
		}

	case *apievents.SessionPrint:
		if evt.DelayMilliseconds < req.StartTime {
			streamer.vtMutex.Lock()
			if _, err := streamer.vt.Write(evt.Data); err != nil {
				s.logger.WarnContext(s.ctx, "failed to write to terminal", "error", err)
			}
			streamer.currentTime = evt.DelayMilliseconds
			streamer.vtMutex.Unlock()
			return true
		}

		if evt.DelayMilliseconds > req.EndTime {
			return false
		}

		streamer.vtMutex.Lock()
		if _, err := streamer.vt.Write(evt.Data); err != nil {
			s.logger.WarnContext(s.ctx, "failed to write to terminal", "error", err)
		}
		streamer.currentTime = evt.DelayMilliseconds
		streamer.vtMutex.Unlock()

		if err := s.writeBinaryEvent(EventTypeSessionPrint, evt.DelayMilliseconds, evt.Data); err != nil {
			s.logger.WarnContext(s.ctx, "failed to send session print", "error", err)
		}

		streamer.lastProcessedTime = evt.DelayMilliseconds

	case *apievents.SessionEnd:
		endTime := int64(evt.EndTime.Sub(evt.StartTime) / time.Millisecond)
		if endTime >= req.StartTime && endTime <= req.EndTime {
			if err := s.writeBinaryEvent(EventTypeSessionEnd, endTime, []byte(evt.EndTime.Format(time.RFC3339))); err != nil {
				s.logger.WarnContext(s.ctx, "failed to send session end", "error", err)
			}
		}
		return false

	case *apievents.Resize:
		parts := strings.Split(evt.TerminalSize, ":")
		if len(parts) == 2 {
			width, err := strconv.Atoi(parts[0])
			if err == nil {
				height, err := strconv.Atoi(parts[1])
				if err == nil {
					streamer.vtMutex.Lock()
					streamer.vt.Resize(width, height)
					streamer.vtMutex.Unlock()

					if err := s.writeBinaryEvent(EventTypeResize, 0, []byte(evt.TerminalSize)); err != nil {
						s.logger.WarnContext(s.ctx, "failed to send resize", "error", err)
					}
				}
			}
		}
	}
	return true
}

func (s *SessionEventsHandler) initStreamer(startTime int64) error {
	s.streamerMutex.Lock()
	defer s.streamerMutex.Unlock()

	if s.streamer != nil && s.streamer.cancel != nil {
		s.streamer.cancel()
	}

	streamCtx, streamCancel := context.WithCancel(s.ctx)

	vt := vt10x.New()
	if startTime > 0 {
		rebuiltVT, err := s.rebuildVTState(startTime)
		if err != nil {
			streamCancel()
			return err
		}
		if rebuiltVT != nil {
			vt = *rebuiltVT
		}
	}

	evts, errs := s.clt.StreamSessionEvents(
		metadata.WithSessionRecordingFormatContext(streamCtx, teleport.PTY),
		session.ID(s.sessionID),
		startTime,
	)

	s.streamer = &sessionEventStreamer{
		events:            evts,
		errors:            errs,
		cancel:            streamCancel,
		vt:                vt,
		lastProcessedTime: startTime,
		currentTime:       startTime,
	}

	return nil
}

func (s *SessionEventsHandler) rebuildVTState(upToTime int64) (*vt10x.Terminal, error) {
	streamCtx, streamCancel := context.WithCancel(s.ctx)
	defer streamCancel()

	vt := vt10x.New()
	evts, errs := s.clt.StreamSessionEvents(
		metadata.WithSessionRecordingFormatContext(streamCtx, teleport.PTY),
		session.ID(s.sessionID),
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
					s.logger.WarnContext(streamCtx, "failed to write to terminal", "error", err)
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
