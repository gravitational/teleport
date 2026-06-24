// Teleport
// Copyright (C) 2023  Gravitational, Inc.
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

package websocketupgradeproto

import (
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/jonboulle/clockwork"
)

// errBrokenPipe is a "broken pipe" error, to be returned by write operations if
// we know that the remote side is closed (reads return io.EOF instead). TCP
// connections actually return ECONNRESET on the first syscall experiencing the
// error, then EPIPE afterwards; we don't bother emulating that detail and
// always return EPIPE instead.
var errBrokenPipe error = syscall.EPIPE

var _ net.Conn = (*Conn)(nil)

// bufferlessConn is a [net.Conn] that's managed externally by interacting with
// the two slices passed to Read and Write. There is no internal buffering.
type bufferlessConn struct {
	// mu protects the rest of the data in the struct.
	mu sync.Mutex

	// cond is a condition variable that uses mu as its Locker. Anything that
	// modifies data that other functions might Wait() on should Broadcast()
	// before unlocking.
	cond sync.Cond

	localAddr  net.Addr
	remoteAddr net.Addr

	readDeadline  deadline
	writeDeadline deadline

	readReq  *request
	writeReq *request
	readErr  error

	// localClosed indicates that Close() has been called; most operations will
	// fail immediately with no effect returning [net.ErrClosed]. Takes priority
	// over just about every other condition.
	localClosed bool

	// remoteClosed indicates that we know that the remote side of the
	// connection is gone; reads will start returning [io.EOF] after exhausting
	// the internal buffer, writes return [syscall.EPIPE].
	remoteClosed bool
}

var _ net.Conn = (*bufferlessConn)(nil)

// Close implements [net.Conn].
func (c *bufferlessConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.closeLocked()
}

func (c *bufferlessConn) closeLocked() error {
	if c.localClosed {
		return net.ErrClosed
	}

	c.localClosed = true
	if c.readDeadline.timer != nil {
		c.readDeadline.timer.Stop()
	}
	if c.writeDeadline.timer != nil {
		c.writeDeadline.timer.Stop()
	}

	if c.readReq != nil && !c.readReq.done {
		c.readReq.err = net.ErrClosed
		c.readReq.done = true
	}
	if c.writeReq != nil && !c.writeReq.done {
		c.writeReq.err = net.ErrClosed
		c.writeReq.done = true
	}
	c.cond.Broadcast()

	return nil
}

// LocalAddr implements [net.Conn].
func (c *bufferlessConn) LocalAddr() net.Addr {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.localAddr
}

// RemoteAddr implements [net.Conn].
func (c *bufferlessConn) RemoteAddr() net.Addr {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.remoteAddr
}

// SetDeadline implements [net.Conn].
func (c *bufferlessConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.localClosed {
		return net.ErrClosed
	}

	c.readDeadline.setDeadlineLocked(t, &c.cond, clockwork.NewRealClock())
	c.writeDeadline.setDeadlineLocked(t, &c.cond, clockwork.NewRealClock())

	return nil
}

// SetReadDeadline implements [net.Conn].
func (c *bufferlessConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.localClosed {
		return net.ErrClosed
	}

	c.readDeadline.setDeadlineLocked(t, &c.cond, clockwork.NewRealClock())

	return nil
}

// SetWriteDeadline implements [net.Conn].
func (c *bufferlessConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.localClosed {
		return net.ErrClosed
	}

	c.writeDeadline.setDeadlineLocked(t, &c.cond, clockwork.NewRealClock())

	return nil
}

// Read implements [net.Conn].
func (c *bufferlessConn) Read(b []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.localClosed {
		return 0, net.ErrClosed
	}

	// a zero-length read should return (0, nil) even when past the read
	// deadline, or if the peer has closed the remote side of the connection
	// and a non-zero-length read would return (0, io.EOF) - this is the
	// behavior from a *net.TCPConn as tested on darwin with go 1.21.4, at
	// least
	if len(b) == 0 {
		return 0, nil
	}

	if c.readDeadline.timeout {
		return 0, os.ErrDeadlineExceeded
	}

	if c.remoteClosed {
		return 0, c.readError()
	}

	for c.readReq != nil && !c.readReq.done {
		c.cond.Wait()
		if c.localClosed {
			return 0, net.ErrClosed
		}
		if c.readDeadline.timeout {
			return 0, os.ErrDeadlineExceeded
		}
		if c.remoteClosed {
			return 0, c.readError()
		}
	}

	req := &request{buf: b}
	c.readReq = req
	c.cond.Broadcast()

	for !req.done {
		if c.readDeadline.timeout {
			req.err = os.ErrDeadlineExceeded
			req.done = true
			c.cond.Broadcast()
			break
		}

		if c.remoteClosed && req.n == 0 {
			req.err = c.readError()
			req.done = true
			c.cond.Broadcast()
			break
		}

		c.cond.Wait()

		if c.localClosed {
			req.err = net.ErrClosed
			req.done = true
			break
		}
	}

	c.readReq = nil
	c.cond.Broadcast()

	if req.err != nil {
		return req.n, req.err
	}

	if req.n == 0 && c.remoteClosed {
		return 0, c.readError()
	}

	return req.n, nil
}

// Write implements [net.Conn].
func (c *bufferlessConn) Write(b []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.localClosed {
		return 0, net.ErrClosed
	}

	if c.writeDeadline.timeout {
		return 0, os.ErrDeadlineExceeded
	}

	if c.remoteClosed {
		return 0, errBrokenPipe
	}

	// deadlines and remote closes make zero-length writes return an error,
	// unlike the behavior on read, as per the behavior of *net.TCPConn on
	// darwin with go 1.21.4
	if len(b) == 0 {
		return 0, nil
	}

	for c.writeReq != nil && !c.writeReq.done {
		c.cond.Wait()

		if c.localClosed {
			return 0, net.ErrClosed
		}

		if c.writeDeadline.timeout {
			return 0, os.ErrDeadlineExceeded
		}

		if c.remoteClosed {
			return 0, errBrokenPipe
		}
	}

	req := &request{buf: b}
	c.writeReq = req
	c.cond.Broadcast()

	for !req.done {
		if c.writeDeadline.timeout {
			req.err = os.ErrDeadlineExceeded
			req.done = true
			c.cond.Broadcast()
			break
		}

		if c.remoteClosed {
			req.err = errBrokenPipe
			req.done = true
			c.cond.Broadcast()
			break
		}

		c.cond.Wait()

		if c.localClosed {
			req.err = net.ErrClosed
			req.done = true
			break
		}
	}

	c.writeReq = nil
	c.cond.Broadcast()

	if req.err != nil {
		return req.n, req.err
	}

	return req.n, nil
}

func (c *bufferlessConn) readError() error {
	if c.readErr != nil {
		return c.readErr
	}
	return io.EOF
}

type request struct {
	buf  []byte
	n    int
	err  error
	done bool
}

// deadline holds the state necessary to handle [net.Conn]-like deadlines.
// Should be paired with a [sync.Cond], whose lock protects access to the data
// inside the deadline, and that will get awakened if and when the timeout is
// reached.
type deadline struct {
	// deadline should not be moved or copied
	_ [0]sync.Mutex

	// timer, if set, is a [time.AfterFunc] timer that sets timeout after
	// reaching the deadline. Initialized on first use.
	timer clockwork.Timer

	// timeout is true if we're past the deadline.
	timeout bool

	// stopped is set if timer is non-nil but it's stopped and ready for reuse.
	stopped bool
}

// setDeadlineLocked sets a new deadline, waking the cond's waiters when the
// deadline is hit (immediately, if the deadline is in the past) and protecting
// its data with cond.L, which is assumed to be held by the caller.
func (d *deadline) setDeadlineLocked(t time.Time, cond *sync.Cond, clock clockwork.Clock) {
	if d.timer != nil {
		for !d.stopped {
			if d.timer.Stop() {
				d.stopped = true
				break
			}
			// we failed to stop the timer, so we have to wait for its callback
			// to finish (as signaled by d.stopped) or it will set the timeout
			// flag after we're done
			cond.Wait()
		}
	}

	if t.IsZero() {
		d.timeout = false
		return
	}

	dt := time.Until(t)

	if dt <= 0 {
		d.timeout = true
		cond.Broadcast()
		return
	}

	d.timeout = false

	if d.timer == nil {
		// the func doesn't know about which time it's supposed to run, so we
		// can reuse this timer by just stopping and resetting it
		d.timer = clock.AfterFunc(dt, func() {
			cond.L.Lock()
			defer cond.L.Unlock()
			d.timeout = true
			d.stopped = true
			cond.Broadcast()
		})
	} else {
		d.timer.Reset(dt)
		d.stopped = false
	}
}

func len64(s []byte) uint64 {
	return uint64(len(s))
}
