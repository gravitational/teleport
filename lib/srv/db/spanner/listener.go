/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package spanner

import (
	"context"
	"io"
	"net"
	"sync/atomic"
)

var _ net.Listener = &fakeListener{}

// fakeListener pretends to be a listener, but all it does is Accept() a single
// connection and then block until one of its contexts is done.
type fakeListener struct {
	conn          net.Conn
	accepted      atomic.Bool
	connCtx       context.Context
	serverContext context.Context
	closeFunc     context.CancelFunc
}

func newFakeListener(serverContext, connCtx context.Context, c net.Conn) *fakeListener {
	closeContext, cancel := context.WithCancel(connCtx)
	return &fakeListener{
		conn:          c,
		connCtx:       closeContext,
		serverContext: serverContext,
		closeFunc:     cancel,
	}
}

func (l *fakeListener) Accept() (net.Conn, error) {
	if l.accepted.Swap(true) {
		select {
		case <-l.connCtx.Done():
		case <-l.serverContext.Done():
		}
		return nil, io.EOF
	}
	return l.conn, nil
}

func (l *fakeListener) Addr() net.Addr {
	return l.conn.LocalAddr()
}

func (l *fakeListener) Close() error {
	l.closeFunc()
	return nil
}
