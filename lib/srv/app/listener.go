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

package app

import (
	"context"
	"errors"
	"net"

	"github.com/gravitational/trace"
)

// errListenerConnServed is used as a signal between listener and Server.
// See listener.Accept for details.
var errListenerConnServed = errors.New("ok: listener conn served")

// listener wraps a net.Conn in a net.Listener interface. This allows passing
// a channel connection from the reverse tunnel subsystem to an HTTP server.
type listener struct {
	connCh    chan net.Conn
	localAddr net.Addr

	closeContext context.Context
	closeFunc    context.CancelFunc
}

// newListener creates a new wrapping listener.
func newListener(ctx context.Context, conn net.Conn) *listener {
	closeContext, closeFunc := context.WithCancel(ctx)

	connCh := make(chan net.Conn, 1)
	connCh <- conn

	return &listener{
		connCh:       connCh,
		localAddr:    conn.LocalAddr(),
		closeContext: closeContext,
		closeFunc:    closeFunc,
	}
}

// Accept returns the connection. An error is returned when this listener
// is closed, its parent context is closed, or the second time it is called.
//
// On the second call, this method returns errListenerConnServed. This will
// trigger the calling http.Serve function to exit gracefully, close this
// listener, and return control to the http.Serve caller. The caller should
// ignore errListenerConnServed and handle all other errors.
func (l *listener) Accept() (net.Conn, error) {
	select {
	case conn, more := <-l.connCh:
		if !more {
			return nil, errListenerConnServed // normal operation signal
		}
		close(l.connCh)
		return conn, nil
	case <-l.closeContext.Done():
		return nil, trace.BadParameter("closing context")
	}
}

// Close closes the connection.
func (l *listener) Close() error {
	l.closeFunc()
	return nil
}

// Addr returns the address of the connection.
func (l *listener) Addr() net.Addr {
	return l.localAddr
}
