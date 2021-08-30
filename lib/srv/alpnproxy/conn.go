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

package alpnproxy

import (
	"io"
	"net"
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
// if err != nil {
//    return err
// }
// Now the client can peek at buff read by conn.Read call.
//
// But to not alter the connection the buff can be prepended to the connection and
// the buffered connection should be sued for further operations.
// conn = newBufferedConn(conn, bytes.NewReader(buff))
// if err := handleConnection(conn); err != nil {
//    return err
// }
//
// The bufferedConn is useful in more complex cases when connection Read call is done in an external library
// Example: Reading the client TLS Hello message TLS termination.
// var hello *tls.ClientHelloInfo
// buff := new(bytes.Buffer)
// tlsConn := tls.Server(readOnlyConn{reader: io.TeeReader(conn, buff)}, &tls.Config{
// 	 GetConfigForClient: func(info *tls.ClientHelloInfo) (*tls.Config, error) {
// 	    hello = info
// 	    return nil, nil
// 	 },
// })
// err := tlsConn.Handshake()
// if hello == nil {
//    return trace.Wrap(err)
// }
//
// Create the bufferedConn with prepended buff obtained from TLS Handshake.
// conn := newBufferedConn(conn, buff)
//
type bufferedConn struct {
	net.Conn
	r io.Reader
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
