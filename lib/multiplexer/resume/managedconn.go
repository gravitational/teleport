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

package resume

import (
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"
)

const (
	receiveBufferSize = 128 * 1024
	replayBufferSize  = 2 * 1024 * 1024
	initialBufferSize = 4096
)

// errBrokenPipe is a "broken pipe" error, to be returned by write operations if
// we know that the remote side is closed (reads return io.EOF instead). TCP
// connections actually return ECONNRESET on the first syscall experiencing the
// error, then EPIPE afterwards; we don't bother emulating that detail and
// always return EPIPE instead.
var errBrokenPipe = syscall.EPIPE

func newManagedConn(localAddr, remoteAddr net.Addr) *managedConn {
	c := &managedConn{
		localAddr:  localAddr,
		remoteAddr: remoteAddr,

		receiveBuffer: buffer{data: make([]byte, initialBufferSize)},
		replayBuffer:  buffer{data: make([]byte, initialBufferSize)},
	}
	c.cond.L = &c.mu

	return c
}

type managedConn struct {
	mu   sync.Mutex
	cond sync.Cond

	localAddr  net.Addr
	remoteAddr net.Addr

	localClosed  bool
	remoteClosed bool

	receiveBuffer buffer
	replayBuffer  buffer

	readDeadline  deadline
	writeDeadline deadline
}

var _ net.Conn = (*managedConn)(nil)

// Close implements [net.Conn].
func (c *managedConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.localClosed {
		return net.ErrClosed
	}

	c.localClosed = true
	c.cond.Broadcast()

	return nil
}

// LocalAddr implements [net.Conn].
func (c *managedConn) LocalAddr() net.Addr {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.localAddr
}

// RemoteAddr implements [net.Conn].
func (c *managedConn) RemoteAddr() net.Addr {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.remoteAddr
}

// SetDeadline implements [net.Conn].
func (c *managedConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.localClosed {
		return net.ErrClosed
	}

	c.readDeadline.setDeadlineLocked(t, &c.cond)
	c.writeDeadline.setDeadlineLocked(t, &c.cond)

	return nil
}

// SetReadDeadline implements [net.Conn].
func (c *managedConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.localClosed {
		return net.ErrClosed
	}

	c.readDeadline.setDeadlineLocked(t, &c.cond)

	return nil
}

// SetWriteDeadline implements [net.Conn].
func (c *managedConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.localClosed {
		return net.ErrClosed
	}

	c.writeDeadline.setDeadlineLocked(t, &c.cond)

	return nil
}

// Read implements [net.Conn].
func (c *managedConn) Read(b []byte) (n int, err error) {
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

	for {
		if c.readDeadline.timeout {
			return 0, os.ErrDeadlineExceeded
		}

		n := c.receiveBuffer.read(b)
		if n > 0 {
			c.cond.Broadcast()
			return n, nil
		}

		if c.remoteClosed {
			return 0, io.EOF
		}

		c.cond.Wait()

		if c.localClosed {
			return 0, net.ErrClosed
		}
	}
}

// Write implements [net.Conn].
func (c *managedConn) Write(b []byte) (n int, err error) {
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

	for {
		if c.replayBuffer.len() < replayBufferSize {
			s := min(replayBufferSize-c.replayBuffer.len(), len64(b))
			c.replayBuffer.append(b[:s])
			b = b[s:]
			n += int(s)
			c.cond.Broadcast()

			if len(b) == 0 {
				return n, nil
			}
		}

		c.cond.Wait()

		if c.localClosed {
			return n, net.ErrClosed
		}

		if c.writeDeadline.timeout {
			return n, os.ErrDeadlineExceeded
		}

		if c.remoteClosed {
			return n, errBrokenPipe
		}
	}
}

func len64(s []byte) uint64 {
	return uint64(len(s))
}

// buffer represents a view of contiguous data in a bytestream, between the
// absolute positions start and end (with 0 being the beginning of the
// bytestream). The byte at absolute position i is data[i % len(data)],
// len(data) is always a power of two (therefore it's always non-empty), and
// len(data) == cap(data).
type buffer struct {
	data  []byte
	start uint64
	end   uint64
}

// bounds returns the indexes of start and end in the current data slice. It's
// possible for left to be greater than right, which happens when the data is
// stored across the end of the slice.
func (w *buffer) bounds() (left, right uint64) {
	return w.start % len64(w.data), w.end % len64(w.data)
}

func (w *buffer) len() uint64 {
	return w.end - w.start
}

func (w *buffer) buffered() ([]byte, []byte) {
	if w.len() == 0 {
		return nil, nil
	}

	left, right := w.bounds()

	if left >= right {
		return w.data[left:], w.data[:right]
	}
	return w.data[left:right], nil
}

func (w *buffer) free() ([]byte, []byte) {
	if w.len() == 0 {
		return w.data, nil
	}

	left, right := w.bounds()

	if left >= right {
		return w.data[right:left], nil
	}
	return w.data[right:], w.data[:left]
}

func (w *buffer) reserve(n uint64) {
	if w.len()+n <= len64(w.data) {
		return
	}

	d1, d2 := w.buffered()
	capacity := len64(w.data) * 2
	for w.len()+n > capacity {
		capacity *= 2
	}
	w.data = make([]byte, capacity)
	left := w.start % capacity
	copy(w.data[left:], d1)
	mid := left + len64(d1)
	if mid > capacity {
		mid -= capacity
	}
	copy(w.data[mid:], d2)
}

func (w *buffer) append(b []byte) {
	w.reserve(len64(b))
	f1, f2 := w.free()
	copy(f2, b[copy(f1, b):])
	w.end += len64(b)
}

func (w *buffer) advance(n uint64) {
	w.start += n
	if w.start > w.end {
		w.end = w.start
	}
}

func (w *buffer) read(b []byte) int {
	d1, d2 := w.buffered()
	n := copy(b, d1)
	n += copy(b[n:], d2)
	w.advance(uint64(n))
	return n
}

type deadline struct {
	timeout bool
	stopped bool
	timer   *time.Timer
}

func (d *deadline) setDeadlineLocked(t time.Time, cond *sync.Cond) {
	if d.timer != nil && !d.stopped {
		if d.timer.Stop() {
			// the happy path: we stopped the timer with plenty of time left, so
			// we prevented the execution of the func, and we can just reuse the
			// timer; unfortunately, timer.Stop() again will return false, so we
			// have to keep an additional boolean flag around
			d.stopped = true
		} else {
			// the timer has fired but the func hasn't completed yet (it'll get
			// stuck acquiring the lock that we're currently holding), so we
			// reset d.timer to tell the func that it should do nothing after
			// acquiring the lock
			d.timer = nil
		}
	}

	// if we got here, either timer is nil and stopped is unset, or timer is not
	// nil but it's not running (and can be reused), and stopped is set

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
		var thisTimer *time.Timer
		thisTimer = time.AfterFunc(dt, func() {
			cond.L.Lock()
			defer cond.L.Unlock()
			if d.timer != thisTimer {
				return
			}
			d.timeout = true
			d.stopped = true
			cond.Broadcast()
		})
		d.timer = thisTimer
	} else {
		d.timer.Reset(dt)
		d.stopped = false
	}
}
