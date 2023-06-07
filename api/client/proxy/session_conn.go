// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"

	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
)

// addr is a [net.Addr] implementation for static tcp addresses.
type addr string

func (a addr) Network() string {
	return "tcp"
}

func (a addr) String() string {
	return string(a)
}

// sessionConn is a [net.Conn] implementation that reads and writes data
// over the standard input and standard output, respectively, of a [tracessh.Session].
type sessionConn struct {
	io.Reader
	session    *tracessh.Session
	localAddr  net.Addr
	remoteAddr net.Addr

	mu sync.Mutex
	w  io.WriteCloser
}

// newSessionConn creates a [net.Conn] for over the provided [tracessh.Session].
func newSessionConn(session *tracessh.Session, local, remote string) (*sessionConn, error) {
	sessionW, err := session.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessionR, err := session.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &sessionConn{
		session:    session,
		Reader:     sessionR,
		w:          sessionW,
		localAddr:  addr(local),
		remoteAddr: addr(remote),
	}, nil
}

func (s *sessionConn) Write(b []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(b)
}

func (s *sessionConn) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return trace.NewAggregate(s.w.Close(), s.session.Close())
}

func (s *sessionConn) LocalAddr() net.Addr {
	return s.localAddr
}

func (s *sessionConn) RemoteAddr() net.Addr {
	return s.remoteAddr
}

func (s *sessionConn) SetDeadline(time.Time) error {
	return nil
}

func (s *sessionConn) SetReadDeadline(time.Time) error {
	return nil
}

func (s *sessionConn) SetWriteDeadline(time.Time) error {
	return nil
}
