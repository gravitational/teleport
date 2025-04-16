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

package streamproto

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// metaMessage is a control message containing one or more payloads.
type metaMessage struct {
	Resize          *remotecommand.TerminalSize `json:"resize,omitempty"`
	ForceTerminate  bool                        `json:"force_terminate,omitempty"`
	ClientHandshake *ClientHandshake            `json:"client_handshake,omitempty"`
	ServerHandshake *ServerHandshake            `json:"server_handshake,omitempty"`
}

// ClientHandshake is the first message sent by a client to inform a server of its intentions.
type ClientHandshake struct {
	Mode types.SessionParticipantMode `json:"mode"`
}

// ServerHandshake is the first message sent by a server to inform a client of the session settings.
type ServerHandshake struct {
	MFARequired bool `json:"mfa_required"`
}

// SessionStream represents one end of the bidirectional session connection.
type SessionStream struct {
	// The underlying websocket connection.
	conn *websocket.Conn

	// A stream of incoming session packets.
	in chan []byte

	// Optionally contains a partially read session packet.
	currentIn []byte

	// A list of resize requests.
	resizeQueue chan *remotecommand.TerminalSize

	// A notification channel for force termination requests.
	forceTerminate chan struct{}

	writeSync   sync.Mutex
	done        chan struct{}
	closeOnce   sync.Once
	closed      int32
	MFARequired bool
	Mode        types.SessionParticipantMode
	isClient    bool
}

// NewSessionStream creates a new session stream.
// The type of the handshake parameter determines if this is the client or server end.
func NewSessionStream(conn *websocket.Conn, handshake any) (*SessionStream, error) {
	s := &SessionStream{
		conn:           conn,
		in:             make(chan []byte),
		done:           make(chan struct{}),
		resizeQueue:    make(chan *remotecommand.TerminalSize, 1),
		forceTerminate: make(chan struct{}),
	}

	clientHandshake, isClient := handshake.(ClientHandshake)
	serverHandshake, ok := handshake.(ServerHandshake)
	s.isClient = isClient

	if !isClient && !ok {
		return nil, trace.BadParameter("Handshake must be either client or server handshake, got %T", handshake)
	}

	if isClient {
		ty, data, err := conn.ReadMessage()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if ty != websocket.TextMessage {
			return nil, trace.Errorf("Expected websocket control message, got %v", ty)
		}

		var msg metaMessage
		if err := utils.FastUnmarshal(data, &msg); err != nil {
			return nil, trace.Wrap(err)
		}

		if msg.ServerHandshake == nil {
			return nil, trace.Errorf("Expected websocket server handshake, got %v", msg)
		}

		s.MFARequired = msg.ServerHandshake.MFARequired
		handshakeMsg := metaMessage{ClientHandshake: &clientHandshake}
		dataClientHandshake, err := utils.FastMarshal(handshakeMsg)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if err := conn.WriteMessage(websocket.TextMessage, dataClientHandshake); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		handshakeMsg := metaMessage{ServerHandshake: &serverHandshake}
		dataServerHandshake, err := utils.FastMarshal(handshakeMsg)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if err := conn.WriteMessage(websocket.TextMessage, dataServerHandshake); err != nil {
			return nil, trace.Wrap(err)
		}

		ty, data, err := conn.ReadMessage()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if ty != websocket.TextMessage {
			return nil, trace.Errorf("Expected websocket control message, got %v", ty)
		}

		var msg metaMessage
		if err := utils.FastUnmarshal(data, &msg); err != nil {
			return nil, trace.Wrap(err)
		}

		if msg.ClientHandshake == nil {
			return nil, trace.Errorf("Expected websocket client handshake")
		}

		s.Mode = msg.ClientHandshake.Mode
	}

	go s.readTask()
	return s, nil
}

func (s *SessionStream) readTask() {
	for {
		defer s.closeOnce.Do(func() { close(s.done) })

		ty, data, err := s.conn.ReadMessage()
		if err != nil {
			if !errors.Is(err, io.EOF) && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
				slog.WarnContext(context.Background(), "Failed to read message from websocket", "error", err)
			}

			var closeErr *websocket.CloseError
			// If it's a close error, we want to send a message to the stdout
			if s.isClient && errors.As(err, &closeErr) && closeErr.Text != "" {
				select {
				case s.in <- []byte(fmt.Sprintf("\r\n---\r\nConnection closed: %v\r\n", closeErr.Text)):
				case <-s.done:
					return
				}
			}

			return
		}

		if ty == websocket.BinaryMessage {
			select {
			case s.in <- data:
			case <-s.done:
				return
			}
		}

		if ty == websocket.TextMessage {
			var msg metaMessage
			if err := utils.FastUnmarshal(data, &msg); err != nil {
				return
			}

			if msg.Resize != nil {
				select {
				case s.resizeQueue <- msg.Resize:
				case <-s.done:
					return
				}
			}

			if msg.ForceTerminate {
				close(s.forceTerminate)
			}
		}

		if ty == websocket.CloseMessage {
			s.conn.Close()
			atomic.StoreInt32(&s.closed, 1)
			return
		}
	}
}

func (s *SessionStream) Read(p []byte) (int, error) {
	if len(s.currentIn) == 0 {
		select {
		case s.currentIn = <-s.in:
		case <-s.done:
			return 0, io.EOF
		}
	}

	n := copy(p, s.currentIn)
	s.currentIn = s.currentIn[n:]
	return n, nil
}

func (s *SessionStream) Write(data []byte) (int, error) {
	s.writeSync.Lock()
	defer s.writeSync.Unlock()

	err := s.conn.WriteMessage(websocket.BinaryMessage, data)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return len(data), nil
}

// Resize sends a resize request to the other party.
func (s *SessionStream) Resize(size *remotecommand.TerminalSize) error {
	msg := metaMessage{Resize: size}
	json, err := utils.FastMarshal(msg)
	if err != nil {
		return trace.Wrap(err)
	}

	s.writeSync.Lock()
	defer s.writeSync.Unlock()
	return trace.Wrap(s.conn.WriteMessage(websocket.TextMessage, json))
}

// ResizeQueue returns a channel that will receive resize requests.
func (s *SessionStream) ResizeQueue() <-chan *remotecommand.TerminalSize {
	return s.resizeQueue
}

// ForceTerminateQueue returns the channel used for force termination requests.
func (s *SessionStream) ForceTerminateQueue() <-chan struct{} {
	return s.forceTerminate
}

// ForceTerminate sends a force termination request to the other end.
func (s *SessionStream) ForceTerminate() error {
	msg := metaMessage{ForceTerminate: true}
	json, err := utils.FastMarshal(msg)
	if err != nil {
		return trace.Wrap(err)
	}

	s.writeSync.Lock()
	defer s.writeSync.Unlock()

	return trace.Wrap(s.conn.WriteMessage(websocket.TextMessage, json))
}

func (s *SessionStream) Done() <-chan struct{} {
	return s.done
}

// Close closes the stream.
func (s *SessionStream) Close() error {
	if atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		err := s.conn.WriteMessage(websocket.CloseMessage, []byte{})
		if err != nil {
			slog.WarnContext(context.Background(), "Failed to gracefully close websocket connection", "error", err)
		}
		t := time.NewTimer(time.Second * 5)
		defer t.Stop()
		select {
		case <-s.done:
		case <-t.C:
			s.conn.Close()
		}
		s.closeOnce.Do(func() { close(s.done) })
	}

	return nil
}
