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

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
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
func NewChConn(conn Conn, ch ssh.ChannelWithDeadlines) *ChConn {
	return newChConn(conn, ch, false)
}

// NewExclusiveChConn returns a new net.Conn implemented over
// SSH channel, whenever this connection closes
func NewExclusiveChConn(conn Conn, ch ssh.ChannelWithDeadlines) *ChConn {
	return newChConn(conn, ch, true)
}

func newChConn(conn Conn, ch ssh.ChannelWithDeadlines, exclusive bool) *ChConn {
	c := &ChConn{
		ChannelWithDeadlines: ch,

		conn:      conn,
		exclusive: exclusive,
	}
	return c
}

// ChConn is a net.Conn like object
// that uses SSH channel
type ChConn struct {
	mu sync.Mutex

	ssh.ChannelWithDeadlines
	conn Conn
	// exclusive indicates that whenever this channel connection
	// is getting closed, the underlying connection is closed as well
	exclusive bool

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
	if err := c.ChannelWithDeadlines.Close(); err != nil {
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

const (
	// ConnectionTypeRequest is a request sent over a SSH channel that returns a
	// boolean which indicates the connection type (direct or tunnel).
	ConnectionTypeRequest = "x-teleport-connection-type"
)
