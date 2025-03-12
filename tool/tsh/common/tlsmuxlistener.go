// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"bufio"
	"context"
	"crypto/tls"
	"net"

	"github.com/gravitational/trace"
)

// connReader is a net.Conn wrapper with additional Peek() method.
type connReader struct {
	reader *bufio.Reader
	net.Conn
}

// newBufferedConn is a connReader constructor.
func newBufferedConn(conn net.Conn) connReader {
	return connReader{
		reader: bufio.NewReader(conn),
		Conn:   conn,
	}
}

// Peek reads n bytes without advancing the reader.
// It's basically a wrapper around (bufio.Reader).Peek()
func (b connReader) Peek(n int) ([]byte, error) {
	return b.reader.Peek(n)
}

// Read returns data from underlying buffer.
func (b connReader) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}

type tlsMuxListener struct {
	listener  net.Listener
	tlsConfig *tls.Config
}

func (m *tlsMuxListener) Accept() (net.Conn, error) {
	conn, err := m.listener.Accept()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	bufConn := newBufferedConn(conn)
	buf, err := bufConn.Peek(1)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	const tlsFirstByte = 0x16

	switch buf[0] {
	case tlsFirstByte:
		logger.DebugContext(context.Background(), "Read first byte, assuming TLS connection")
		return tls.Server(bufConn, m.tlsConfig), nil
	default:
		return bufConn, nil
	}
}

func (m *tlsMuxListener) Close() error {
	return m.listener.Close()
}

func (m *tlsMuxListener) Addr() net.Addr {
	return m.listener.Addr()
}

// NewTLSMuxListener returns new multiplexing listener. If the incoming connection appears to use TLS, a TLS listener will serve it.
// Otherwise, it will be served raw.
func NewTLSMuxListener(listener net.Listener, tlsConfig *tls.Config) net.Listener {
	return &tlsMuxListener{
		listener:  listener,
		tlsConfig: tlsConfig,
	}
}
