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

package tdp

import (
	"bufio"
	"io"
	"net"
	"sync"

	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/trace"
)

// Conn is a desktop protocol connection.
// It converts between a stream of bytes (io.ReadWriter) and a stream of
// Teleport Desktop Protocol (TDP) messages.
type Conn struct {
	rw        io.ReadWriter
	bufr      *bufio.Reader
	closeOnce sync.Once

	// localAddr and remoteAddr will be set if rw is
	// a conn that provides these fields
	localAddr  net.Addr
	remoteAddr net.Addr
}

// NewConn creates a new Conn on top of a ReadWriter, for example a TCP
// connection. If the provided ReadWriter also implements srv.TrackingConn,
// then its LocalAddr() and RemoteAddr() will apply to this Conn.
func NewConn(rw io.ReadWriter) *Conn {
	c := &Conn{
		rw:   rw,
		bufr: bufio.NewReader(rw),
	}

	if tc, ok := rw.(srv.TrackingConn); ok {
		c.localAddr = tc.LocalAddr()
		c.remoteAddr = tc.RemoteAddr()
	}

	return c
}

// Close closes the connection if the underlying reader can be closed.
func (c *Conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		if closer, ok := c.rw.(io.Closer); ok {
			err = closer.Close()
		}
	})
	return err
}

// InputMessage reads the next incoming message from the connection.
func (c *Conn) InputMessage() (Message, error) {
	m, err := decode(c.bufr)
	return m, trace.Wrap(err)
}

// OutputMessage sends a message to the connection.
func (c *Conn) OutputMessage(m Message) error {
	buf, err := m.Encode()
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := c.rw.Write(buf); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// SendError is a convenience function for sending an error message.
func (c *Conn) SendError(message string) error {
	return c.OutputMessage(Error{Message: message})
}

// LocalAddr returns local address
func (c *Conn) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr returns remote address
func (c *Conn) RemoteAddr() net.Addr {
	return c.remoteAddr
}
