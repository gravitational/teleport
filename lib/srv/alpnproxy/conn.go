/*
Copyright 2021 Gravitational, Inc.

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

package alpnproxy

import (
	"crypto/tls"
	"encoding/binary"
	"io"
	"math"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// readOnlyConn allows to only for Read operation. Other net.Conn operation will be discarded.
type readOnlyConn struct {
	reader io.Reader
}

func (conn readOnlyConn) Read(p []byte) (int, error)         { return conn.reader.Read(p) }
func (conn readOnlyConn) Write(p []byte) (int, error)        { return 0, io.ErrClosedPipe }
func (conn readOnlyConn) Close() error                       { return nil }
func (conn readOnlyConn) LocalAddr() net.Addr                { return &utils.NetAddr{} }
func (conn readOnlyConn) RemoteAddr() net.Addr               { return &utils.NetAddr{} }
func (conn readOnlyConn) SetDeadline(t time.Time) error      { return nil }
func (conn readOnlyConn) SetReadDeadline(t time.Time) error  { return nil }
func (conn readOnlyConn) SetWriteDeadline(t time.Time) error { return nil }

// NewPingConn returns a ping connection wrapping the provided net.Conn.
func NewPingConn(conn *tls.Conn) *PingConn {
	return &PingConn{Conn: conn}
}

// PingConn wraps a *tls.Conn and add ping capabilities to it, including the
// `WritePing` function and `Read` (which excludes ping packets).
//
// When using this connection, the packets written will contain an initial data:
// the packet size. When reading, this information is taken into account, but it
// is not returned to the caller.
//
// Ping messages have a packet size of zero and are produced only when
// `WritePing` is called. On `Read`, any Ping packet is discarded.
type PingConn struct {
	//net.Conn
	*tls.Conn

	muRead  sync.Mutex
	muWrite sync.Mutex

	// currentSize size of bytes of the current packet.
	currentSize uint32
}

// Read reads content from the underlying connection, discarding any ping
// messages it finds.
func (c *PingConn) Read(p []byte) (int, error) {
	c.muRead.Lock()
	defer c.muRead.Unlock()

	err := c.discardPingReads()
	if err != nil {
		return 0, trace.Wrap(err)
	}

	// Check if the current size is larger than the provided buffer.
	readSize := c.currentSize
	if c.currentSize > uint32(len(p)) {
		readSize = uint32(len(p))
	}

	n, err := c.Conn.Read(p[:readSize])
	c.currentSize -= uint32(n)

	return n, err
}

// WritePing writes the ping packet to the connection.
func (c *PingConn) WritePing() error {
	c.muWrite.Lock()
	defer c.muWrite.Unlock()

	return binary.Write(c.Conn, binary.BigEndian, uint32(0))
}

// discardPingReads reads from the wrapped net.Conn until it encounters a
// non-ping packet.
func (c *PingConn) discardPingReads() error {
	for c.currentSize == 0 {
		err := binary.Read(c.Conn, binary.BigEndian, &c.currentSize)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// Write writes provided content to the underlying connection with proper
// protocol fields.
func (c *PingConn) Write(p []byte) (int, error) {
	c.muWrite.Lock()
	defer c.muWrite.Unlock()

	// Avoid overflow when casting data length. It is only present to avoid
	// panicking if the size cannot be cast. Callers should handle packet length
	// limits, such as protocol implementations and audits.
	if uint64(len(p)) > math.MaxUint32 {
		return 0, trace.BadParameter("invalid content size, max size permitted is %d", uint64(math.MaxUint32))
	}

	size := uint32(len(p))
	if size == 0 {
		return 0, nil
	}

	// Write packet size.
	if err := binary.Write(c.Conn, binary.BigEndian, size); err != nil {
		return 0, trace.Wrap(err)
	}

	// Iterate until everything is written.
	var written int
	for written < len(p) {
		n, err := c.Conn.Write(p)
		written += n

		if err != nil {
			return written, trace.Wrap(err)
		}
	}

	return written, nil
}
