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

	"github.com/jonboulle/clockwork"
)

const (
	receiveBufferSize = 128 * 1024
	sendBufferSize    = 2 * 1024 * 1024
	initialBufferSize = 4096
)

// errBrokenPipe is a "broken pipe" error, to be returned by write operations if
// we know that the remote side is closed (reads return io.EOF instead). TCP
// connections actually return ECONNRESET on the first syscall experiencing the
// error, then EPIPE afterwards; we don't bother emulating that detail and
// always return EPIPE instead.
var errBrokenPipe = syscall.EPIPE

func newManagedConn() *managedConn {
	c := new(managedConn)
	c.cond.L = &c.mu
	return c
}

// managedConn is a [net.Conn] that's managed externally by interacting with its
// two internal buffers, one for each direction, which also keep track of the
// absolute positions in the bytestream.
type managedConn struct {
	// mu protects the rest of the data in the struct.
	mu sync.Mutex

	// cond is a condition variable that uses mu as its Locker. Anything that
	// modifies data that other functions might Wait() on should Broadcast()
	// before unlocking.
	cond sync.Cond

	localAddr  net.Addr
	remoteAddr net.Addr

	// localClosed indicates that Close() has been called; most operations will
	// fail immediately with no effect returning [net.ErrClosed]. Takes priority
	// over just about every other condition.
	localClosed bool

	// remoteClosed indicates that we know that the remote side of the
	// connection is gone; reads will start returning [io.EOF] after exhausting
	// the internal buffer, writes return [syscall.EPIPE].
	remoteClosed bool

	receiveBuffer buffer
	sendBuffer    buffer

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

	c.readDeadline.setDeadlineLocked(t, &c.cond, clockwork.NewRealClock())
	c.writeDeadline.setDeadlineLocked(t, &c.cond, clockwork.NewRealClock())

	return nil
}

// SetReadDeadline implements [net.Conn].
func (c *managedConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.localClosed {
		return net.ErrClosed
	}

	c.readDeadline.setDeadlineLocked(t, &c.cond, clockwork.NewRealClock())

	return nil
}

// SetWriteDeadline implements [net.Conn].
func (c *managedConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.localClosed {
		return net.ErrClosed
	}

	c.writeDeadline.setDeadlineLocked(t, &c.cond, clockwork.NewRealClock())

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
		if c.sendBuffer.len() < sendBufferSize {
			s := min(sendBufferSize-c.sendBuffer.len(), len64(b))
			c.sendBuffer.append(b[:s])
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

// buffered returns the currently used areas of the internal buffer, in order.
// It's not possible for the second slice to be nonempty if the first slice is
// empty. The total length of the slices is equal to w.len().
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

// free returns the currently unused areas of the internal buffer, in order.
// It's not possible for the second slice to be nonempty if the first slice is
// empty. The total length of the slices is equal to len(w.data)-w.len().
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

// reserve ensures that the buffer has a given amount of free space,
// reallocating its internal buffer as needed. After reserve(n), the two slices
// returned by free have total length at least n.
func (w *buffer) reserve(n uint64) {
	n += w.len()
	if n <= len64(w.data) {
		return
	}

	newCapacity := max(len64(w.data)*2, initialBufferSize)
	for n > newCapacity {
		newCapacity *= 2
	}

	d1, d2 := w.buffered()
	w.data = make([]byte, newCapacity)
	w.end = w.start

	// this is less efficient than copying the data manually, but almost all
	// uses of buffer will eventually hit a maximum buffer size anyway
	w.append(d1)
	w.append(d2)
}

// append copies the slice to the tail of the buffer, resizing it if necessary.
// Writing to the slices returned by free() and appending them in order will not
// result in any memory copy (if the buffer hasn't been reallocated).
func (w *buffer) append(b []byte) {
	w.reserve(len64(b))
	f1, f2 := w.free()
	// after reserve(n), len(f1)+len(f2) >= n, so this is guaranteed to work
	copy(f2, b[copy(f1, b):])
	w.end += len64(b)
}

// advance will discard bytes from the head of the buffer, advancing its start
// position. Advancing past the end causes the end to be pushed forwards as
// well, such that an empty buffer advanced by n ends up with start = end = n.
func (w *buffer) advance(n uint64) {
	w.start += n
	if w.start > w.end {
		w.end = w.start
	}
}

// read will attempt to fill the slice with as much data from the buffer,
// advancing the start position of the buffer to match. Returns the amount of
// bytes copied in the slice.
func (w *buffer) read(b []byte) int {
	d1, d2 := w.buffered()
	n := copy(b, d1)
	n += copy(b[n:], d2)
	w.advance(uint64(n))
	return n
}

// deadline holds the state necessary to handle [net.Conn]-like deadlines.
// Should be paired with a [sync.Cond], whose lock protects access to the data
// inside the deadline, and that will get awakened if and when the timeout is
// reached.
type deadline struct {
	// deadline should not be moved or copied
	_ [0]sync.Mutex

	// timeout is true if we're past the deadline.
	timeout bool

	// stopped is set if timer is non-nil but it's stopped and ready for reuse.
	stopped bool
	// timer, if set, is a [time.AfterFunc] timer that sets timeout after
	// reaching the deadline. Initialized on first use.
	timer clockwork.Timer
}

// setDeadlineLocked sets a new deadline, waking the cond's waiters when the
// deadline is hit (immediately, if the deadline is in the past) and protecting
// its data with cond.L, which is assumed to be held by the caller.
func (d *deadline) setDeadlineLocked(t time.Time, cond *sync.Cond, clock clockwork.Clock) {
	if d.timer != nil {
		for {
			if d.stopped {
				break
			}
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
