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
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/lib/utils"
)

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

// NewConnProxy creates a bidirectional proxy to copy messages between the client and server connection.
// It accepts an optional interceptor to intercept server messages.
func NewConnProxy(client, server io.ReadWriteCloser, serverInterceptor Interceptor) *ConnProxy {
	return &ConnProxy{
		client:            NewConn(client),
		server:            NewConn(server),
		serverInterceptor: serverInterceptor,
	}
}

// ConnProxy does a bidirectional copy between the connection to the client and the mTLS connection to the server.
type ConnProxy struct {
	// client is a connection to the client (browser/Connect).
	client *Conn
	// server is a connection to the server (Windows Desktop Service).
	server *Conn
	// serverInterceptor intercepts messages received from the serve.
	serverInterceptor Interceptor
}

// Interceptor intercepts messages on the connection. It should return
// the [potentially modified] message in order to pass it on to the
// other end of the connection, or nil to prevent the message from
// being forwarded.
type Interceptor func(conn *Conn, message Message) (Message, error)

// SendToClient sends a message to the client and blocks until the operation completes.
func (c *ConnProxy) SendToClient(message Message) error {
	err := c.client.WriteMessage(message)
	return trace.Wrap(err)
}

// SendToServer sends a message to the server and blocks until the operation completes.
func (c *ConnProxy) SendToServer(message Message) error {
	err := c.server.WriteMessage(message)
	return trace.Wrap(err)
}

// Run starts proxying the connection.
func (c *ConnProxy) Run() error {
	var errs errgroup.Group

	closeAll := sync.OnceFunc(func() {
		c.client.Close()
		c.server.Close()
	})
	defer closeAll()

	// Run a goroutine to read TDP messages from the Windows
	// agent and write them to client.
	errs.Go(func() error {
		defer closeAll()

		// We avoid using io.Copy here, as we want to make sure
		// each TDP message is sent as a unit so that a single
		// 'message' event is emitted in the JS TDP client.
		// Internal buffer of io.Copy could split one message
		// into multiple downstreamConn.Send() calls.
		// We don't care about the content of the message, we just
		// need to split the stream into individual messages and
		// write them to the client
		for {
			msg, err := c.server.ReadMessage()

			if err := c.handleError(err); err != nil {
				return err
			}

			if c.serverInterceptor != nil {
				msg, err = c.serverInterceptor(c.server, msg)
				if err != nil {
					return trace.Wrap(err)
				}
			}
			if msg != nil {
				err := c.SendToClient(msg)
				if err != nil {
					return trace.Wrap(err)
				}
			}
		}
	})

	// Run a goroutine to read TDP messages coming from the client
	// and pass them on to the Windows agent.
	errs.Go(func() error {
		defer closeAll()

		for {
			msg, err := c.client.ReadMessage()
			if err := c.handleError(err); err != nil {
				return err
			}

			if err := c.SendToServer(msg); err != nil {
				return trace.Wrap(err)
			}
		}
	})

	// Wait for all goroutines to finish
	if err := errs.Wait(); err != nil && !utils.IsOKNetworkError(err) {
		return trace.Wrap(err)
	}

	return nil
}

func (c *ConnProxy) handleError(err error) error {
	if err == nil {
		return nil
	}
	if utils.IsOKNetworkError(err) {
		return trace.Wrap(err)
	}
	isFatal := IsFatalErr(err)
	severity := SeverityError
	if !isFatal {
		severity = SeverityWarning
	}
	sendErr := c.SendToClient(Alert{Message: err.Error(), Severity: severity})

	// If the error wasn't fatal, and we successfully
	// sent it back to the client, continue.
	if !isFatal && sendErr == nil {
		return nil
	}

	// If the error was fatal, or we failed to send it back
	// to the client, return it and end the session.
	if sendErr != nil {
		err = sendErr
	}
	return trace.Wrap(err)
}
