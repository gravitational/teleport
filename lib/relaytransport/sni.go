// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package relaytransport

import (
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/credentials"
)

type SNIDispatchFunc = func(serverName string, transcript *bytes.Buffer, rawConn net.Conn) (dispatched bool)

// SNIDispatchTransportCredentials is a wrapper around a
// [credentials.TransportCredentials] that reads a TLS ClientHello from each
// client connection and optionally dispatches the connection through a custom
// function based on the Server Name Indication in the ClientHello, without
// interacting with the connection otherwise.
type SNIDispatchTransportCredentials struct {
	_ struct{}

	credentials.TransportCredentials

	// DispatchFunc should return true if the connection with the given server
	// name should not be handled by the gRPC server. The transcript buffer
	// contains the data that was received from the connection (containing the
	// TLS ClientHello and potentially more data).
	DispatchFunc SNIDispatchFunc
}

var _ credentials.TransportCredentials = (*SNIDispatchTransportCredentials)(nil)

// Clone implements [credentials.TransportCredentials].
func (s *SNIDispatchTransportCredentials) Clone() credentials.TransportCredentials {
	return &SNIDispatchTransportCredentials{
		TransportCredentials: s.TransportCredentials.Clone(),
		DispatchFunc:         s.DispatchFunc,
	}
}

// ServerHandshake implements [credentials.TransportCredentials].
func (s *SNIDispatchTransportCredentials) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	// the deadline for rawConn was set by the grpc server machinery based on
	// the server connection timeout, we should not apply additional timeouts

	transcript := new(bytes.Buffer)
	var serverName string
	var receivedHello bool
	if err := tls.Server(
		&transcripterConn{conn: rawConn, transcript: transcript},
		&tls.Config{
			GetConfigForClient: func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
				serverName = chi.ServerName
				receivedHello = true
				return nil, io.EOF
			},
		},
	).Handshake(); !receivedHello && err != nil {
		_ = rawConn.Close()
		return nil, nil, trace.Wrap(err)
	}

	if s.DispatchFunc(serverName, transcript, rawConn) {
		return nil, nil, credentials.ErrConnDispatched
	}

	if transcript.Len() > 0 {
		rawConn = &transcriptedConn{
			Conn:       rawConn,
			transcript: transcript,
		}
	}

	return s.TransportCredentials.ServerHandshake(rawConn)
}

// transcripterConn is a net.Conn that also stores the data that was read from a
// real net.Conn in a transcript, and for which every other operation is a noop.
type transcripterConn struct {
	transcript *bytes.Buffer
	conn       net.Conn
}

var _ net.Conn = (*transcripterConn)(nil)

// Close implements [net.Conn].
func (p *transcripterConn) Close() error { return nil }

// Read implements [net.Conn].
func (p *transcripterConn) Read(b []byte) (n int, err error) {
	n, err = p.conn.Read(b)
	_, _ = p.transcript.Write(b[:n])
	return n, err
}

// LocalAddr implements [net.Conn].
func (p *transcripterConn) LocalAddr() net.Addr { return p.conn.LocalAddr() }

// RemoteAddr implements [net.Conn].
func (p *transcripterConn) RemoteAddr() net.Addr { return p.conn.RemoteAddr() }

// SetDeadline implements [net.Conn].
func (p *transcripterConn) SetDeadline(t time.Time) error { return nil }

// SetReadDeadline implements [net.Conn].
func (p *transcripterConn) SetReadDeadline(t time.Time) error { return nil }

// SetWriteDeadline implements [net.Conn].
func (p *transcripterConn) SetWriteDeadline(t time.Time) error { return nil }

// Write implements [net.Conn].
func (p *transcripterConn) Write(b []byte) (n int, err error) { return len(b), nil }

// transcriptedConn is a net.Conn whose incoming data begins with the data in a
// transcript.
type transcriptedConn struct {
	net.Conn
	transcript *bytes.Buffer
}

var _ net.Conn = (*transcriptedConn)(nil)

// Read implements [net.Conn].
func (p *transcriptedConn) Read(b []byte) (n int, err error) {
	if p.transcript == nil {
		return p.Conn.Read(b)
	}

	// this is improper behavior since closing the Conn will not cause read
	// errors until the transcript is exhausted and reads past the deadline will
	// still succeed, but this net.Conn implementation is very much intended for
	// a buffer containing a TLS ClientHello that gets fully read almost
	// immediately anyway
	n = copy(b, p.transcript.Next(len(b)))
	if p.transcript.Len() < 1 {
		p.transcript = nil
	}

	return n, nil
}
