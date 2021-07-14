/*
Copyright 2020-2021 Gravitational, Inc.

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
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"time"
)

// readHelloMessageWithoutTLSTermination allows to read a ClientHelloInfo message without termination of
// incoming TLS connection. After calling readHelloMessageWithoutTLSTermination function a returned
// net.Conn should be used for further operation.
func readHelloMessageWithoutTLSTermination(conn net.Conn) (*tls.ClientHelloInfo, net.Conn, error) {
	buff := new(bytes.Buffer)
	var hello *tls.ClientHelloInfo
	err := tls.Server(readOnlyConn{reader: io.TeeReader(conn, buff)}, &tls.Config{
		GetConfigForClient: func(info *tls.ClientHelloInfo) (*tls.Config, error) {
			hello = info
			return nil, nil
		},
	}).Handshake()
	if hello == nil {
		return nil, nil, err
	}
	return hello, newConnWrap(conn, buff), nil
}

// newConnWrap creates new instance of connWrap.
func newConnWrap(conn net.Conn, header io.Reader) *connWrap {
	return &connWrap{
		Conn: conn,
		r:    io.MultiReader(header, conn),
	}
}

// connWrap allows to inject additional reader that will be drained during Read call reading from net.Conn.
type connWrap struct {
	net.Conn
	r io.Reader
}

func (conn connWrap) Read(p []byte) (int, error) { return conn.r.Read(p) }

// readOnlyConn allows to only for Read operation. Other net.Conn operation will be discarded.
type readOnlyConn struct {
	reader io.Reader
}

func (conn readOnlyConn) Read(p []byte) (int, error)         { return conn.reader.Read(p) }
func (conn readOnlyConn) Write(p []byte) (int, error)        { return 0, io.ErrClosedPipe }
func (conn readOnlyConn) Close() error                       { return nil }
func (conn readOnlyConn) LocalAddr() net.Addr                { return nil }
func (conn readOnlyConn) RemoteAddr() net.Addr               { return nil }
func (conn readOnlyConn) SetDeadline(t time.Time) error      { return nil }
func (conn readOnlyConn) SetReadDeadline(t time.Time) error  { return nil }
func (conn readOnlyConn) SetWriteDeadline(t time.Time) error { return nil }
