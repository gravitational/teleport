/*
Copyright 2015 Gravitational, Inc.

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

package utils

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// PipeNetConn implemetns net.Conn from io.Reader,io.Writer and io.Closer
type PipeNetConn struct {
	reader     io.Reader
	writer     io.Writer
	closer     io.Closer
	localAddr  net.Addr
	remoteAddr net.Addr
}

// NewPipeNetConn returns a net.Conn like object
// using Pipe as an underlying implementation over reader, writer and closer
func NewPipeNetConn(reader io.Reader,
	writer io.Writer,
	closer io.Closer,
	fakelocalAddr net.Addr,
	fakeRemoteAddr net.Addr) *PipeNetConn {

	return &PipeNetConn{
		reader:     reader,
		writer:     writer,
		closer:     closer,
		localAddr:  fakelocalAddr,
		remoteAddr: fakeRemoteAddr,
	}
}

func (nc *PipeNetConn) Read(buf []byte) (n int, e error) {
	return nc.reader.Read(buf)
}

func (nc *PipeNetConn) Write(buf []byte) (n int, e error) {
	return nc.writer.Write(buf)
}

func (nc *PipeNetConn) Close() error {
	if nc.closer != nil {
		return nc.closer.Close()
	}
	return nil
}

func (nc *PipeNetConn) LocalAddr() net.Addr {
	return nc.localAddr
}

func (nc *PipeNetConn) RemoteAddr() net.Addr {
	return nc.remoteAddr
}

func (nc *PipeNetConn) SetDeadline(t time.Time) error {
	return nil
}

func (nc *PipeNetConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (nc *PipeNetConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// DualPipeAddrConn creates a net.Pipe to connect a client and a server. The
// two net.Conn instances are wrapped in an addrConn which holds the source and
// destination addresses.
func DualPipeNetConn(srcAddr net.Addr, dstAddr net.Addr) (*PipeNetConn, *PipeNetConn) {
	server, client := net.Pipe()

	serverConn := NewPipeNetConn(server, server, server, dstAddr, srcAddr)
	clientConn := NewPipeNetConn(client, client, client, srcAddr, dstAddr)

	return serverConn, clientConn
}

// NewChConn returns a new net.Conn implemented over
// SSH channel
func NewChConn(conn ssh.Conn, ch ssh.Channel) *ChConn {
	return &ChConn{
		Channel: ch,
		conn:    conn,
	}
}

// NewExclusiveChConn returns a new net.Conn implemented over
// SSH channel, whenever this connection closes
func NewExclusiveChConn(conn ssh.Conn, ch ssh.Channel) *ChConn {
	return &ChConn{
		Channel:   ch,
		conn:      conn,
		exclusive: true,
	}
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

// UseTunnel makes a channel request asking for the type of connection. If
// the other side does not respond (older cluster) or takes to long to
// respond, be on the safe side and assume it's not a tunnel connection.
func (c *ChConn) UseTunnel() bool {
	responseCh := make(chan bool, 1)

	go func() {
		ok, err := c.SendRequest(ConnectionTypeRequest, true, nil)
		if err != nil {
			responseCh <- false
			return
		}
		responseCh <- ok
	}()

	select {
	case response := <-responseCh:
		return response
	case <-time.After(1 * time.Second):
		logrus.Debugf("Timed out waiting for response: returning false.")
		return false
	}
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

// CancelableChConn is a wrapped SSH channel connection that supports deadlines.
type CancelableChConn struct {
	// ChConn is the base wrapped SSH channel connection.
	*ChConn
	// reader is the part of the pipe that clients read from.
	reader net.Conn
	// writer is the part of the pipe that receives data from SSH channel.
	writer net.Conn
}

// NewCancelableChConn returns a new instance of wrapped SSH channel connection
// that supports deadlines.
func NewCancelableChConn(conn ssh.Conn, ch ssh.Channel) *CancelableChConn {
	reader, writer := net.Pipe()
	c := &CancelableChConn{
		ChConn: NewChConn(conn, ch),
		reader: reader,
		writer: writer,
	}
	// Start copying from the SSH channel to the writer part of the pipe. The
	// clients are reading from the reader part of the pipe (see Read below).
	//
	// This goroutine stops when either the SSH channel closes or this
	// connection is closed e.g. by a http.Server (see Close below).
	go io.Copy(writer, ch)
	return c
}

// Read reads from the channel.
func (c *CancelableChConn) Read(data []byte) (int, error) {
	return c.reader.Read(data)
}

// SetDeadline sets the channel connection read/write deadlines.
func (c *CancelableChConn) SetDeadline(t time.Time) error {
	return c.reader.SetDeadline(t)
}

// SetReadDeadline sets the channel connection read deadline.
func (c *CancelableChConn) SetReadDeadline(t time.Time) error {
	return c.reader.SetReadDeadline(t)
}

// Closes closes all parts of the connection.
func (c *CancelableChConn) Close() error {
	var errors []error
	if err := c.ChConn.Close(); err != nil {
		errors = append(errors, err)
	}
	if err := c.reader.Close(); err != nil {
		errors = append(errors, err)
	}
	if err := c.writer.Close(); err != nil {
		errors = append(errors, err)
	}
	return trace.NewAggregate(errors...)
}

const (
	// ConnectionTypeRequest is a request sent over a SSH channel that returns a
	// boolean which indicates the connection type (direct or tunnel).
	ConnectionTypeRequest = "x-teleport-connection-type"
)
