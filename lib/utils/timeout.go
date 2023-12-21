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

package utils

import (
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// ObeyIdleTimeout wraps an existing network connection, closing it if data
// isn't read often enough. The connection will be closed even if Read is never
// called, or if it's called on the underlying connection instead of the
// returned one.
func ObeyIdleTimeout(conn net.Conn, timeout time.Duration) net.Conn {
	return obeyIdleTimeoutClock(conn, timeout, clockwork.NewRealClock())
}

// obeyIdleTimeoutClock is [ObeyIdleTimeout] but lets the caller specify an
// arbitrary [clockwork.Clock] to be used for the timer.
func obeyIdleTimeoutClock(conn net.Conn, timeout time.Duration, clock clockwork.Clock) net.Conn {
	return &timeoutConn{
		Conn:    conn,
		timeout: timeout,
		watchdog: clock.AfterFunc(timeout, func() {
			conn.Close()
		}),
	}
}

type timeoutConn struct {
	net.Conn

	timeout time.Duration

	mu       sync.Mutex
	watchdog clockwork.Timer
}

func (c *timeoutConn) pet() {
	c.mu.Lock()
	defer c.mu.Unlock()
	// if the timer has already fired the underlying net.Conn has been closed or
	// will be closed shortly anyway
	if c.watchdog.Stop() {
		c.watchdog.Reset(c.timeout)
	}
}

// NetConn returns the underlying [net.Conn].
func (c *timeoutConn) NetConn() net.Conn {
	return c.Conn
}

// Close implements [io.Closer] and [net.Conn] by closing the underlying
// connection and then stopping the watchdog, if it's still running.
func (c *timeoutConn) Close() error {
	err := c.Conn.Close()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.watchdog.Stop()
	return trace.Wrap(err)
}

// Read implements [io.Reader] and [net.Conn], petting the watchdog timer if any
// data is successfully read.
func (c *timeoutConn) Read(p []byte) (n int, err error) {
	n, err = c.Conn.Read(p)
	if n > 0 {
		c.pet()
	}
	// avoid trace.Wrap to maintain the exact errors from the underlying
	// connection (like io.EOF)
	return n, err
}
