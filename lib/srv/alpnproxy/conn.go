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

package alpnproxy

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/utils"
)

// newBufferedConn creates new instance of bufferedConn.
func newBufferedConn(conn net.Conn, header io.Reader) *bufferedConn {
	return &bufferedConn{
		Conn: conn,
		r:    io.MultiReader(header, conn),
	}
}

// bufferedConn allows injecting additional reader that will be drained during Read call reading from net.Conn.
// Is used when part of the data on a connection has already been read.
//
// Example: Prepend Read buff to the connection.
// conn, err := conn.Read(buff)
//
//	if err != nil {
//	   return err
//	}
//
// Now the client can peek at buff read by conn.Read call.
//
// But to not alter the connection the buff can be prepended to the connection and
// the buffered connection should be sued for further operations.
// conn = newBufferedConn(conn, bytes.NewReader(buff))
//
//	if err := handleConnection(conn); err != nil {
//	   return err
//	}
//
// The bufferedConn is useful in more complex cases when connection Read call is done in an external library
// Example: Reading the client TLS Hello message TLS termination.
// var hello *tls.ClientHelloInfo
// buff := new(bytes.Buffer)
//
//	tlsConn := tls.Server(readOnlyConn{reader: io.TeeReader(conn, buff)}, &tls.Config{
//		 GetConfigForClient: func(info *tls.ClientHelloInfo) (*tls.Config, error) {
//		    hello = info
//		    return nil, nil
//		 },
//	})
//
// err := tlsConn.HandshakeContext(ctx)
//
//	if hello == nil {
//	   return trace.Wrap(err)
//	}
//
// Create the bufferedConn with prepended buff obtained from TLS Handshake.
// conn := newBufferedConn(conn, buff)
type bufferedConn struct {
	net.Conn
	r io.Reader
}

// NetConn returns the underlying net.Conn.
func (conn bufferedConn) NetConn() net.Conn {
	return conn.Conn
}

func (conn bufferedConn) Read(p []byte) (int, error) { return conn.r.Read(p) }

// readOnlyConn allows to only for Read operation. Other net.Conn operation will be discarded.
type readOnlyConn struct {
	reader io.Reader
}

func (conn readOnlyConn) Read(p []byte) (int, error)         { return conn.reader.Read(p) }
func (conn readOnlyConn) Write(p []byte) (int, error)        { return 0, io.ErrClosedPipe }
func (conn readOnlyConn) Close() error                       { return nil }
func (conn readOnlyConn) LocalAddr() net.Addr                { return &utils.NetAddr{} }
func (conn readOnlyConn) RemoteAddr() net.Addr               { return &utils.NetAddr{} }
func (conn readOnlyConn) SetDeadline(t time.Time) error      { return nil }
func (conn readOnlyConn) SetReadDeadline(t time.Time) error  { return nil }
func (conn readOnlyConn) SetWriteDeadline(t time.Time) error { return nil }

type reportingConn struct {
	net.Conn
	closeOnce func() error
}

// newReportingConn returns a net.Conn that wraps the input conn, increments the
// active connections gauge on creation, and decreases the active connections
// gauge on close.
func newReportingConn(conn net.Conn, clientALPN string, connSource string) net.Conn {
	proxyActiveConnections.WithLabelValues(clientALPN, connSource).Inc()
	return &reportingConn{
		Conn: conn,
		closeOnce: sync.OnceValue(func() error {
			proxyActiveConnections.WithLabelValues(clientALPN, connSource).Dec()
			// Do not wrap.
			return conn.Close()
		}),
	}
}

// NetConn returns the underlying net.Conn.
func (c *reportingConn) NetConn() net.Conn {
	return c.Conn
}

func (c *reportingConn) Close() error {
	return c.closeOnce()
}
