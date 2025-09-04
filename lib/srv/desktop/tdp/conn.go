/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package tdp

import (
	"bufio"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/gravitational/trace"
)

type MessageReader interface {
	ReadMessage() (Message, error)
}

type MessageWriter interface {
	WriteMessage(Message) error
}

type MessageReadWriter interface {
	MessageReader
	MessageWriter
}

type MessageReadWriteCloser interface {
	MessageReadWriter
	Close() error
}

// Conn is a desktop protocol connection.
// It converts between a stream of bytes (io.ReadWriter) and a stream of
// Teleport Desktop Protocol (TDP) messages.
type Conn struct {
	rwc       io.ReadWriteCloser
	writeMu   sync.Mutex
	bufr      *bufio.Reader
	closeOnce sync.Once

	// OnSend is an optional callback that is invoked when a TDP message
	// is sent on the wire. It is passed both the raw bytes and the encoded
	// message.
	OnSend func(m Message, b []byte)

	// OnRecv is an optional callback that is invoked when a TDP message
	// is received on the wire.
	OnRecv func(m Message)

	// localAddr and remoteAddr will be set if rw is
	// a conn that provides these fields
	localAddr  net.Addr
	remoteAddr net.Addr
}

// NewConn creates a new Conn on top of a ReadWriter, for example a TCP
// connection. If the provided ReadWriter also implements srv.TrackingConn,
// then its LocalAddr() and RemoteAddr() will apply to this Conn.
func NewConn(rwc io.ReadWriteCloser) *Conn {
	c := &Conn{
		rwc:  rwc,
		bufr: bufio.NewReader(rwc),
	}

	if tc, ok := rwc.(srvTrackingConn); ok {
		c.localAddr = tc.LocalAddr()
		c.remoteAddr = tc.RemoteAddr()
	}

	return c
}

// srvTrackingConn should be kept in sync with
// lib/srv.TrackingConn. It is duplicated here
// to avoid placing a dependency on the lib/srv
// package, which is incompatible with Windows.
type srvTrackingConn interface {
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
	Close() error
}

// Close closes the connection if the underlying reader can be closed.
func (c *Conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		err = c.rwc.Close()
	})
	return err
}

// NextMessageType peaks at the next incoming message without
// consuming it.
func (c *Conn) NextMessageType() (MessageType, error) {
	b, err := c.bufr.ReadByte()
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if err := c.bufr.UnreadByte(); err != nil {
		return 0, trace.Wrap(err)
	}
	return MessageType(b), nil
}

// ReadMessage reads the next incoming message from the connection.
func (c *Conn) ReadMessage() (Message, error) {
	m, err := decode(c.bufr)
	if c.OnRecv != nil {
		c.OnRecv(m)
	}
	return m, trace.Wrap(err)
}

// WriteMessage sends a message to the connection.
func (c *Conn) WriteMessage(m Message) error {
	buf, err := m.Encode()
	if err != nil {
		return trace.Wrap(err)
	}

	c.writeMu.Lock()
	_, err = c.rwc.Write(buf)
	c.writeMu.Unlock()

	if c.OnSend != nil {
		c.OnSend(m, buf)
	}
	return trace.Wrap(err)
}

// ReadClientScreenSpec reads the next message from the connection, expecting
// it to be a ClientScreenSpec. If it is not, an error is returned.
func (c *Conn) ReadClientScreenSpec() (*ClientScreenSpec, error) {
	m, err := c.ReadMessage()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	spec, ok := m.(ClientScreenSpec)
	if !ok {
		return nil, trace.BadParameter("expected ClientScreenSpec, got %T", m)
	}

	return &spec, nil
}

// SendNotification is a convenience function for sending a Notification message.
func (c *Conn) SendNotification(message string, severity Severity) error {
	return c.WriteMessage(Alert{Message: message, Severity: severity})
}

// LocalAddr returns local address
func (c *Conn) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr returns remote address
func (c *Conn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

// IsNonFatalErr returns whether or not an error arising from
// the tdp package should be interpreted as fatal or non-fatal
// for an ongoing TDP connection.
func IsNonFatalErr(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, clipDataMaxLenErr) ||
		errors.Is(err, stringMaxLenErr) ||
		errors.Is(err, fileReadWriteMaxLenErr) ||
		errors.Is(err, mfaDataMaxLenErr)
}

// IsFatalErr returns the inverse of IsNonFatalErr
// (except for if err == nil, for which both functions return false)
func IsFatalErr(err error) bool {
	if err == nil {
		return false
	}

	return !IsNonFatalErr(err)
}

// Interceptor intercepts messages. It should return
// the [potentially modified] message in order to pass it on to the
// other end of the connection, or nil to prevent the message from
// being forwarded.
type Interceptor func(message Message) (Message, error)

// ReadWriteInterceptor wraps an existing 'MessageReadWriteCloser' and runs the
// provided interceptor functions in the read and/or write paths. Allows callers
// to snoop and modify messages as they pass through the 'MessageReadWriteCloser'.
type ReadWriteInterceptor struct {
	// The underlying read/writer to intercept messages on
	src MessageReadWriteCloser
	// The interceptor to run in the read path (allowed to be nil)
	read Interceptor
	// The interceptor to run in the read path (allowed to be nil)
	write Interceptor
}

// NewReadWriteInterceptor creates a new 'ReadWriteInterceptor' that intercepts messages on 'src'.
func NewReadWriteInterceptor(src MessageReadWriteCloser, readIntercept, writeIntercept Interceptor) *ReadWriteInterceptor {
	return &ReadWriteInterceptor{
		src:   src,
		read:  readIntercept,
		write: writeIntercept,
	}
}

// WriteMessage passes the message to the write interceptor (if provded)
// for omition or modification before writing the message to the underlying
// writer.
func (i *ReadWriteInterceptor) WriteMessage(m Message) error {
	var err error
	if i.write != nil {
		m, err = i.write(m)
		if err != nil {
			return err
		}
	}
	// The interceptor is allowed to return a nil message
	if m != nil {
		return i.src.WriteMessage(m)
	}
	return nil
}

// ReadMessage reads from the underlying reader and passes them to the
// read interceptor (if provided) for omition or modification before
// returning the next message.
func (i *ReadWriteInterceptor) ReadMessage() (Message, error) {
	var m Message
	var err error
	for m == nil && err == nil {
		m, err = i.src.ReadMessage()
		if err == nil && i.read != nil {
			m, err = i.read(m)
		}
	}
	return m, err
}

// Close calls close on the underlying 'MessageReadWriteCloser'
func (i *ReadWriteInterceptor) Close() error {
	return i.src.Close()
}

// messageCopy behaves similarly to io.Copy except it deals with Message types.
// It reads messages from 'src' and writes them to 'dst' until an error is received.
// It does *not* forward an EOF received from the reader, but returns nil in the happy path.
func messageCopy(dst MessageWriter, src MessageReader) error {
	var err error
	var m Message
	for err == nil {
		m, err = src.ReadMessage()
		if err == nil {
			err = dst.WriteMessage(m)
		}
	}

	if errors.Is(err, io.EOF) {
		err = nil
	}
	return nil
}

// ConnProxy handles bi-directional copying of messages from server <-> client.
type ConnProxy struct {
	server MessageReadWriteCloser
	client MessageReadWriteCloser
}

// NewConnProxy returns a new ConnProxy.
func NewConnProxy(client, server MessageReadWriteCloser) ConnProxy {
	return ConnProxy{
		server: server,
		client: client,
	}
}

// Run handles bi-directional copying of messages from server <-> client until
// an IO error occurs (or EOF is received from either side). It always calls
// 'close' on both streams before exiting and returns any errors occurred from
// reading, writing, or closing both streams.
func (c *ConnProxy) Run() error {
	newCopyFunc := func(dst, src MessageReadWriteCloser, e *error) func() {
		return func() {
			defer func() {
				// Call close on the other side of the connection.
				// This should wake them up.
				*e = errors.Join(*e, dst.Close())
			}()
			// Copy from server to client
			*e = messageCopy(dst, src)
		}
	}
	g := sync.WaitGroup{}
	// Copy in both directions
	var clientToServerErr, serverToClientErr error
	g.Go(newCopyFunc(c.client, c.server, &serverToClientErr))
	g.Go(newCopyFunc(c.server, c.client, &clientToServerErr))
	g.Wait()

	return errors.Join(clientToServerErr, serverToClientErr)
}
