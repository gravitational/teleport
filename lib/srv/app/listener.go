/*
Copyright 2020 Gravitational, Inc.

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

package app

import (
	"context"
	"net"

	"github.com/gravitational/trace"
)

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

// Accept returns the connection.
func (l *listener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connCh:
		return conn, nil
	case <-l.closeContext.Done():
		return nil, trace.BadParameter("closing context")
	}
}

// Close closes the connection.
func (l *listener) Close() error {
	l.closeFunc()
	return l.closeContext.Err()
}

// Addr returns the address of the connection.
func (l *listener) Addr() net.Addr {
	return l.localAddr
}
