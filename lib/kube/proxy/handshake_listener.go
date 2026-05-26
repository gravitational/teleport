/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
)

// handshakeBoundedTLSListener wraps a plaintext net.Listener and performs the
// TLS handshake on each accepted connection in a worker goroutine with an explicit timeout.
// Only connections whose handshake completed within the timeout are delivered via Accept.
//
// This exists because net/http's Server derives its TLS handshake deadline from
// min(ReadHeaderTimeout, ReadTimeout, WriteTimeout) (see Go stdlib net/http/server.go's tlsHandshakeTimeout).
// The kube TLS server intentionally leaves ReadTimeout/WriteTimeout unset,
// so the implicit handshake bound becomes ReadHeaderTimeout (60s).
//
// By completing the handshake here with a bounded context, we keep the generous ReadHeaderTimeout
// for legitimate header reads while capping the pre-handshake goroutine hold time.
// http.Server still calls HandshakeContext on the returned *tls.Conn,
// but tls.Conn tracks handshake state and the second call is a no-op.
type handshakeBoundedTLSListener struct {
	inner     net.Listener
	tlsConfig *tls.Config
	timeout   time.Duration

	out       chan acceptItem
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
}

type acceptItem struct {
	conn net.Conn
	err  error
}

// newHandshakeBoundedTLSListener wraps inner so that each accepted connection
// must complete a TLS handshake within timeout. Connections that fail or stall
// are closed and never surface through Accept.
func newHandshakeBoundedTLSListener(inner net.Listener, tlsConfig *tls.Config, timeout time.Duration) net.Listener {
	l := &handshakeBoundedTLSListener{
		inner:     inner,
		tlsConfig: tlsConfig,
		timeout:   timeout,
		out:       make(chan acceptItem),
		done:      make(chan struct{}),
	}
	go l.acceptLoop()
	return l
}

func (l *handshakeBoundedTLSListener) acceptLoop() {
	for {
		conn, err := l.inner.Accept()
		if err != nil {
			select {
			case l.out <- acceptItem{err: err}:
			case <-l.done:
			}
			return
		}
		go l.handshake(conn)
	}
}

func (l *handshakeBoundedTLSListener) handshake(conn net.Conn) {
	tlsConn := tls.Server(conn, l.tlsConfig)
	ctx, cancel := context.WithTimeout(context.Background(), l.timeout)
	defer cancel()
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return
	}
	select {
	case l.out <- acceptItem{conn: tlsConn}:
	case <-l.done:
		_ = tlsConn.Close()
	}
}

// Accept blocks until a handshake-completed connection is available or the listener is closed.
func (l *handshakeBoundedTLSListener) Accept() (net.Conn, error) {
	select {
	case item := <-l.out:
		if item.err != nil {
			return nil, item.err
		}
		return item.conn, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

// Close shuts down the listener. Idempotent.
func (l *handshakeBoundedTLSListener) Close() error {
	l.closeOnce.Do(func() {
		close(l.done)
		err := l.inner.Close()
		if err != nil && !errors.Is(err, net.ErrClosed) {
			l.closeErr = trace.Wrap(err)
		}
	})
	return l.closeErr
}

func (l *handshakeBoundedTLSListener) Addr() net.Addr {
	return l.inner.Addr()
}
