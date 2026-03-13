// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package debug

import (
	"io"
	"net"
)

// ConnListener is a net.Listener that accepts connections pushed into it
// via HandleConnection. It bridges reverse tunnel connections to a gRPC
// server by implementing both net.Listener (for grpc.Server.Serve) and
// ServerHandler (for the reverse tunnel agent pool).
type ConnListener struct {
	conns chan net.Conn
	done  chan struct{}
}

// NewConnListener creates a new ConnListener.
func NewConnListener() *ConnListener {
	return &ConnListener{
		conns: make(chan net.Conn),
		done:  make(chan struct{}),
	}
}

// HandleConnection pushes a connection to the listener for the gRPC
// server to accept. It implements the reversetunnel.ServerHandler interface.
func (l *ConnListener) HandleConnection(conn net.Conn) {
	select {
	case l.conns <- conn:
	case <-l.done:
		conn.Close()
	}
}

// Accept waits for a connection to be pushed via HandleConnection.
func (l *ConnListener) Accept() (net.Conn, error) {
	select {
	case conn, ok := <-l.conns:
		if !ok {
			return nil, io.EOF
		}
		return conn, nil
	case <-l.done:
		return nil, io.EOF
	}
}

// Close shuts down the listener.
func (l *ConnListener) Close() error {
	select {
	case <-l.done:
	default:
		close(l.done)
	}
	return nil
}

// Addr returns a placeholder address.
func (l *ConnListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1")}
}
