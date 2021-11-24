/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package streamproto

import (
	"sync"

	"github.com/gorilla/websocket"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"k8s.io/client-go/tools/remotecommand"
)

type SessionStream struct {
	conn        *websocket.Conn
	in          chan []byte
	currentIn   []byte
	resizeQueue chan *remotecommand.TerminalSize
	writeSync   sync.Mutex
	closeC      chan struct{}
	closedC     sync.Once
}

func NewSessionStream(conn *websocket.Conn) *SessionStream {
	s := &SessionStream{
		conn:   conn,
		in:     make(chan []byte),
		closeC: make(chan struct{}),
	}

	go s.readTask()
	return s
}

func (s *SessionStream) readTask() {
	for {
		ty, data, err := s.conn.ReadMessage()
		if err != nil {
			return
		}

		if ty == websocket.BinaryMessage {
			s.in <- data
		}

		if ty == websocket.TextMessage {
			var msg *remotecommand.TerminalSize
			if err := utils.FastUnmarshal(data, msg); err != nil {
				return
			}

			s.resizeQueue <- msg
		}

		if ty == websocket.CloseMessage {
			s.closedC.Do(func() { close(s.closeC) })
		}
	}
}

func (s *SessionStream) Read(p []byte) (int, error) {
	if s.currentIn == nil {
		s.currentIn = <-s.in
	}

	n := copy(p, s.currentIn)
	s.currentIn = s.currentIn[n:]
	return n, nil
}

func (s *SessionStream) Write(data []byte) (int, error) {
	s.writeSync.Lock()
	defer s.writeSync.Unlock()

	return len(data), s.conn.WriteMessage(websocket.BinaryMessage, data)
}

func (s *SessionStream) Resize(size *remotecommand.TerminalSize) error {
	json, err := utils.FastMarshal(size)
	if err != nil {
		return trace.Wrap(err)
	}

	s.writeSync.Lock()
	defer s.writeSync.Unlock()

	return trace.Wrap(s.conn.WriteMessage(websocket.TextMessage, json))
}

func (s *SessionStream) ResizeQueue() chan *remotecommand.TerminalSize {
	return s.resizeQueue
}

func (s *SessionStream) WaitOnClose() {
	<-s.closeC
}

func (s *SessionStream) Close() error {
	s.closedC.Do(func() { close(s.closeC) })
	return trace.Wrap(s.conn.Close())
}
