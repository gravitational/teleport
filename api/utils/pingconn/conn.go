/*
Copyright 2022 Gravitational, Inc.

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

package pingconn

import (
	"encoding/binary"
	"net"
	"sync"
)

// New returns a ping connection wrapping the provided net.Conn.
func New(conn net.Conn) *pingConn {
	return &pingConn{Conn: conn}
}

// pingConn wraps a net.Conn and add ping capabilities to it, including the
// `WritePing` function and `Read` (which excludes ping packets).
//
// When using this connection, the packets written will contain an initial data:
// the packet size. When reading, this information is taken into account, but it
// is not returned to the caller.
//
// Ping messages have a packet size of zero and are produced only when
// `WritePing` is called. On `Read`, any Ping packet is discarded.
type pingConn struct {
	net.Conn

	muRead  sync.Mutex
	muWrite sync.Mutex

	// bytesRead number of bytes already read from the current packet.
	bytesRead int
	// currentSize size of bytes of the current packet.
	currentSize int32
}

func (c *pingConn) Read(p []byte) (int, error) {
	c.muRead.Lock()
	defer c.muRead.Unlock()

	err := c.discardPingReads()
	if err != nil {
		return 0, err
	}

	// Check if the current size is larger than the provided buffer.
	readSize := c.currentSize
	if c.currentSize > int32(len(p)) {
		readSize = int32(len(p))
	}

	n, err := c.Conn.Read(p[:readSize])
	c.bytesRead += n

	// Check if it has read everything.
	if int32(c.bytesRead) >= c.currentSize {
		c.bytesRead = 0
		c.currentSize = 0
	}

	return n, err
}

// discardPingReads reads from the wrapped net.Conn until it encounters a
// non-ping packet.
func (c *pingConn) discardPingReads() error {
	if c.bytesRead > 0 {
		return nil
	}

	for c.currentSize == 0 {
		err := binary.Read(c.Conn, binary.LittleEndian, &c.currentSize)
		if err != nil {
			return err
		}
	}

	return nil
}

// WritePing writes the ping packet to the connection.
func (c *pingConn) WritePing() error {
	c.muWrite.Lock()
	defer c.muWrite.Unlock()

	return binary.Write(c.Conn, binary.LittleEndian, int32(0))
}

func (c *pingConn) Write(p []byte) (int, error) {
	c.muWrite.Lock()
	defer c.muWrite.Unlock()

	size := int32(len(p))
	if size == 0 {
		return 0, nil
	}

	// Write packet size.
	if err := binary.Write(c.Conn, binary.LittleEndian, size); err != nil {
		return 0, err
	}

	// Iterate until everything is written.
	var written int
	for written < len(p) {
		n, err := c.Conn.Write(p)
		written += n

		if err != nil {
			return written, err
		}
	}

	return written, nil
}
