/*
Copyright 2015-2021 Gravitational, Inc.

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

package sshutils

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
)

type Conn interface {
	io.Closer
	// RemoteAddr returns the remote address for this connection.
	RemoteAddr() net.Addr
	// LocalAddr returns the local address for this connection.
	LocalAddr() net.Addr
}

// NewChConn returns a new net.Conn implemented over
// SSH channel
func NewChConn(conn Conn, ch ssh.Channel) *ChConn {
	return newChConn(conn, ch, false)
}

// NewExclusiveChConn returns a new net.Conn implemented over
// SSH channel, whenever this connection closes
func NewExclusiveChConn(conn Conn, ch ssh.Channel) *ChConn {
	return newChConn(conn, ch, true)
}

func newChConn(conn Conn, ch ssh.Channel, exclusive bool) *ChConn {
	reader, writer := net.Pipe()
	c := &ChConn{
		Channel:   ch,
		conn:      conn,
		exclusive: exclusive,
		reader:    reader,
		writer:    writer,
	}
	// Start copying from the SSH channel to the writer part of the pipe. The
	// clients are reading from the reader part of the pipe (see Read below).
	//
	// This goroutine stops when either the SSH channel closes or this
	// connection is closed e.g. by a http.Server (see Close below).
	go func() {
		io.Copy(writer, ch)
		// propagate EOF across the pipe to the read half.
		writer.Close()
	}()
	return c
}

// ChConn is a net.Conn like object
// that uses SSH channel
type ChConn struct {
	mu sync.Mutex

	ssh.Channel
	conn Conn
	// exclusive indicates that whenever this channel connection
	// is getting closed, the underlying connection is closed as well
	exclusive bool

	// reader is the part of the pipe that clients read from.
	reader net.Conn
	// writer is the part of the pipe that receives data from SSH channel.
	writer net.Conn

	// closed prevents double-close
	closed bool
}

// Close closes channel and if the ChConn is exclusive, connection as well
func (c *ChConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	var errors []error
	if err := c.Channel.Close(); err != nil {
		errors = append(errors, err)
	}
	if err := c.reader.Close(); err != nil {
		errors = append(errors, err)
	}
	if err := c.writer.Close(); err != nil {
		errors = append(errors, err)
	}
	// Exclusive means close the underlying SSH connection as well.
	if !c.exclusive {
		return trace.NewAggregate(errors...)
	}
	if err := c.conn.Close(); err != nil {
		errors = append(errors, err)
	}
	return trace.NewAggregate(errors...)
}

// LocalAddr returns a local address of a connection
// Uses underlying net.Conn implementation
func (c *ChConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns a remote address of a connection
// Uses underlying net.Conn implementation
func (c *ChConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// Read reads from the channel.
func (c *ChConn) Read(data []byte) (int, error) {
	n, err := c.reader.Read(data)
	// A lot of code relies on "use of closed network connection" error to
	// gracefully handle terminated connections so convert the closed pipe
	// error to it.
	if err != nil && err == io.ErrClosedPipe {
		return n, trace.ConnectionProblem(err, constants.UseOfClosedNetworkConnection)
	}
	// Do not wrap the error to avoid masking the underlying error such as
	// timeout error which is returned when read deadline is exceeded.
	return n, err
}

// SetDeadline sets a connection deadline.
func (c *ChConn) SetDeadline(t time.Time) error {
	return c.reader.SetDeadline(t)
}

// SetReadDeadline sets a connection read deadline.
func (c *ChConn) SetReadDeadline(t time.Time) error {
	return c.reader.SetReadDeadline(t)
}

// SetWriteDeadline sets write deadline on a connection
// ignored for the channel connection
func (c *ChConn) SetWriteDeadline(t time.Time) error {
	return nil
}

const (
	// ConnectionTypeRequest is a request sent over a SSH channel that returns a
	// boolean which indicates the connection type (direct or tunnel).
	ConnectionTypeRequest = "x-teleport-connection-type"
)
