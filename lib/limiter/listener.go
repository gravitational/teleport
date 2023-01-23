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
func NewListener(ln net.Listener, limiter *ConnectionsLimiter) *Listener {
	return &Listener{
		Listener: ln,
		limiter:  limiter,
	}
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

// Close releases the connection from the limiter and closes the
// underlying [net.Conn].
func (w *wrappedConn) Close() error {
	w.once.Do(w.release)
	return trace.Wrap(w.Conn.Close())
}
