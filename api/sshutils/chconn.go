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
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// NewChConn returns a new net.Conn implemented over
// SSH channel
func NewChConn(conn ssh.Conn, ch ssh.Channel) *ChConn {
	c := &ChConn{}
	c.Channel = ch
	c.conn = conn
	return c
}

// NewExclusiveChConn returns a new net.Conn implemented over
// SSH channel, whenever this connection closes
func NewExclusiveChConn(conn ssh.Conn, ch ssh.Channel) *ChConn {
	c := &ChConn{
		exclusive: true,
	}
	c.Channel = ch
	c.conn = conn
	return c
}

// ChConn is a net.Conn like object
// that uses SSH channel
type ChConn struct {
	mu sync.Mutex

	ssh.Channel
	conn ssh.Conn
	// exclusive indicates that whenever this channel connection
	// is getting closed, the underlying connection is closed as well
	exclusive bool
}

// Close closes channel and if the ChConn is exclusive, connection as well
func (c *ChConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.Channel.Close()
	if !c.exclusive {
		return trace.Wrap(err)
	}
	err2 := c.conn.Close()
	return trace.NewAggregate(err, err2)
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

// SetDeadline sets a connection deadline
// ignored for the channel connection
func (c *ChConn) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline sets a connection read deadline
// ignored for the channel connection
func (c *ChConn) SetReadDeadline(t time.Time) error {
	return nil
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
