/*
Copyright 2015 Gravitational, Inc.

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

package utils

import (
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
)

// ObeyIdleTimeout wraps an existing network connection, closing it if data
// isn't read often enough. The connection will be closed even if Read is never
// called, or if it's called on the underlying connection instead of the
// returned one.
func ObeyIdleTimeout(conn net.Conn, timeout time.Duration) net.Conn {
	return obeyIdleTimeoutFunc(conn, timeout, time.AfterFunc)
}

// obeyIdleTimeoutFunc is [ObeyIdleTimeout] but lets the caller specify a
// callable to replace [time.AfterFunc]. Useful in tests, to use a
// [clockwork.Clock]'s AfterFunc method instead.
func obeyIdleTimeoutFunc[AFT afterFuncTimer](
	conn net.Conn, timeout time.Duration, afterFunc func(time.Duration, func()) AFT,
) net.Conn {
	return &timeoutConn[AFT]{
		Conn:    conn,
		timeout: timeout,
		watchdog: afterFunc(timeout, func() {
			conn.Close()
		}),
	}
}

// afterFuncTimer follows the semantics of a [*time.Timer] returned by
// [time.AfterFunc].
type afterFuncTimer interface {
	Reset(d time.Duration) bool
	Stop() bool
}

type timeoutConn[AFT afterFuncTimer] struct {
	net.Conn

	timeout time.Duration

	mu       sync.Mutex
	watchdog AFT
}

func (c *timeoutConn[_]) pet() {
	c.mu.Lock()
	defer c.mu.Unlock()
	// if the timer has already fired the underlying net.Conn has been closed or
	// will be closed shortly anyway
	if c.watchdog.Stop() {
		c.watchdog.Reset(c.timeout)
	}
}

// NetConn returns the underlying [net.Conn].
func (c *timeoutConn[_]) NetConn() net.Conn {
	return c.Conn
}

// Close implements [io.Closer] and [net.Conn] by closing the underlying
// connection and then stopping the watchdog, if it's still running.
func (c *timeoutConn[_]) Close() error {
	err := c.Conn.Close()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.watchdog.Stop()
	return trace.Wrap(err)
}

// Read implements [io.Reader] and [net.Conn], petting the watchdog timer if any
// data is successfully read.
func (c *timeoutConn[_]) Read(p []byte) (n int, err error) {
	n, err = c.Conn.Read(p)
	if n > 0 {
		c.pet()
	}
	// avoid trace.Wrap to maintain the exact errors from the underlying
	// connection (like io.EOF)
	return n, err
}
