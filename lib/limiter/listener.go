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

package limiter

import (
	"net"
	"sync"

	"github.com/gravitational/trace"
)

// Listener wraps a [net.Listener] and applies connection limiting
// per client to all connections that are accepted.
type Listener struct {
	net.Listener
	limiter *ConnectionsLimiter
}

// NewListener creates a [Listener] that enforces the limits of
// the provided [ConnectionsLimiter] on the all connections accepted
// by the provided [net.Listener].
func NewListener(ln net.Listener, limiter *ConnectionsLimiter) (*Listener, error) {
	if ln == nil {
		return nil, trace.BadParameter("listener cannot be nil")
	}

	return &Listener{
		Listener: ln,
		limiter:  limiter,
	}, nil
}

// Accept waits for and returns the next connection to the listener
// if the limiter is able to acquire a connection. If not, and the max number
// of connections has been exceeded then a [trace.LimitExceeded] error
// is returned.
func (l *Listener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	remoteAddr, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return nil, trace.NewAggregate(err, conn.Close())
	}

	if err := l.limiter.AcquireConnection(remoteAddr); err != nil {
		// Aggregating the error here makes trace.IsLimitExceeded
		// return false, which poses problems for consumers relying
		// on that to determine if the connection was prevented.
		_ = conn.Close()
		return nil, trace.Wrap(err)
	}

	return &wrappedConn{
		Conn: conn,
		release: func() {
			l.limiter.ReleaseConnection(remoteAddr)
		},
	}, nil

}

// wrappedConn allows connections accepted via the [Listener] to decrement
// the connection count.
type wrappedConn struct {
	net.Conn

	once    sync.Once
	release func()
}

// NetConn return the underlying net.Conn.
func (w *wrappedConn) NetConn() net.Conn {
	return w.Conn
}

// Close releases the connection from the limiter and closes the
// underlying [net.Conn].
func (w *wrappedConn) Close() error {
	w.once.Do(w.release)
	return trace.Wrap(w.Conn.Close())
}
