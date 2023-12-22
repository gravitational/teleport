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

package resumption

import (
	"io"
	"net"
	"os"
	"time"

	"github.com/jonboulle/clockwork"
)

// NewPipe returns a pair of [net.Conn]s connected to each other, with buffers
// in each direction. Local and remote addrs are from the point of view of the
// first Conn.
func NewPipe(localAddr, remoteAddr net.Addr) (net.Conn, net.Conn) {
	c := &managerConn{
		c: managedConn{
			localAddr:  localAddr,
			remoteAddr: remoteAddr,
		},
	}
	c.c.cond.L = &c.c.mu
	return &c.c, c
}

type managerConn struct {
	c managedConn

	readDeadline  deadline
	writeDeadline deadline
}

var _ net.Conn = (*managerConn)(nil)

// Close implements [net.Conn].
func (m *managerConn) Close() error {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	if m.c.remoteClosed {
		return net.ErrClosed
	}

	m.c.remoteClosed = true
	if m.readDeadline.timer != nil {
		m.readDeadline.timer.Stop()
	}
	if m.writeDeadline.timer != nil {
		m.writeDeadline.timer.Stop()
	}

	m.c.cond.Broadcast()

	return nil
}

// LocalAddr implements [net.Conn].
func (m *managerConn) LocalAddr() net.Addr {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	return m.c.remoteAddr
}

// RemoteAddr implements [net.Conn].
func (m *managerConn) RemoteAddr() net.Addr {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	return m.c.localAddr
}

// SetDeadline implements [net.Conn].
func (m *managerConn) SetDeadline(t time.Time) error {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	if m.c.remoteClosed {
		return net.ErrClosed
	}

	m.readDeadline.setDeadlineLocked(t, &m.c.cond, clockwork.NewRealClock())
	m.writeDeadline.setDeadlineLocked(t, &m.c.cond, clockwork.NewRealClock())

	return nil
}

// SetReadDeadline implements [net.Conn].
func (m *managerConn) SetReadDeadline(t time.Time) error {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	if m.c.remoteClosed {
		return net.ErrClosed
	}

	m.readDeadline.setDeadlineLocked(t, &m.c.cond, clockwork.NewRealClock())

	return nil
}

// SetWriteDeadline implements [net.Conn].
func (m *managerConn) SetWriteDeadline(t time.Time) error {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	if m.c.remoteClosed {
		return net.ErrClosed
	}

	m.writeDeadline.setDeadlineLocked(t, &m.c.cond, clockwork.NewRealClock())

	return nil
}

// Read implements [net.Conn].
func (m *managerConn) Read(b []byte) (n int, err error) {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	if m.c.remoteClosed {
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
		if m.readDeadline.timeout {
			return 0, os.ErrDeadlineExceeded
		}

		n := m.c.sendBuffer.read(b)
		if n > 0 {
			m.c.cond.Broadcast()
			return n, nil
		}

		if m.c.localClosed {
			return 0, io.EOF
		}

		m.c.cond.Wait()

		if m.c.remoteClosed {
			return 0, net.ErrClosed
		}
	}
}

// Write implements [net.Conn].
func (m *managerConn) Write(b []byte) (n int, err error) {
	m.c.mu.Lock()
	defer m.c.mu.Unlock()

	if m.c.remoteClosed {
		return 0, net.ErrClosed
	}

	if m.writeDeadline.timeout {
		return 0, os.ErrDeadlineExceeded
	}

	if m.c.localClosed {
		return 0, errBrokenPipe
	}

	// deadlines and remote closes make zero-length writes return an error,
	// unlike the behavior on read, as per the behavior of *net.TCPConn on
	// darwin with go 1.21.4
	if len(b) == 0 {
		return 0, nil
	}

	for {
		s := m.c.receiveBuffer.write(b, sendBufferSize)
		if s > 0 {
			m.c.cond.Broadcast()
			b = b[s:]
			n += int(s)

			if len(b) == 0 {
				return n, nil
			}
		}

		m.c.cond.Wait()

		if m.c.remoteClosed {
			return n, net.ErrClosed
		}

		if m.writeDeadline.timeout {
			return n, os.ErrDeadlineExceeded
		}

		if m.c.localClosed {
			return n, errBrokenPipe
		}
	}
}
