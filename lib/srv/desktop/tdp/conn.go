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

	"github.com/gravitational/trace"
)

// Conn is a desktop protocol connection.
// It converts between a stream of bytes (io.ReadWriter) and a stream of
// Teleport Desktop Protofol (TDP) messages.
type Conn struct {
	rw   io.ReadWriter
	bufr *bufio.Reader

	// OnSend is an optional callback that is invoked when a TDP message
	// is sent on the wire. It is passed both the raw bytes and the encoded
	// message.
	OnSend func(m Message, b []byte)
}

// NewConn creates a new Conn on top of a ReadWriter, for example a TCP
// connection.
func NewConn(rw io.ReadWriter) *Conn {
	return &Conn{
		rw:   rw,
		bufr: bufio.NewReader(rw),
	}
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

	_, err = c.rw.Write(buf)

	if c.OnSend != nil {
		c.OnSend(m, buf)
	}

	return trace.Wrap(err)
}
