package deskproto

import (
	"bufio"
	"io"

	"github.com/gravitational/trace"
)

// Conn is a desktop protocol connection.
type Conn struct {
	rw   io.ReadWriter
	bufr *bufio.Reader
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
	if _, err := c.rw.Write(buf); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
