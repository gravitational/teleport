/*

 Copyright 2022 Gravitational, Inc.

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

package protocol

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"

	"github.com/gravitational/trace"
	"github.com/siddontang/go-mysql/client"
	"github.com/siddontang/go-mysql/mysql"
	mysqlpacket "github.com/siddontang/go-mysql/packet"
)

// FetchMySQLVersionInternal connects to a MySQL instance with provided dialer and tries to read the server
// version from initial handshake message. Error is returned in case of connection failure or when MySQL
// returns ERR package.
func FetchMySQLVersionInternal(ctx context.Context, dialer client.Dialer, databaseURI string) (string, error) {
	conn, err := dialer(ctx, "tcp", databaseURI)
	if err != nil {
		return "", trace.ConnectionProblem(err, "failed to connect to MySQL")
	}
	defer conn.Close()

	// Set connection deadline if passed context has it.
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetReadDeadline(deadline); err != nil {
			return "", trace.Wrap(err)
		}
	}

	connBuf := newBufferedConn(ctx, conn)
	pkgType, err := connBuf.Peek(5)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// ref: https://dev.mysql.com/doc/internals/en/mysql-packet.html
	//      https://dev.mysql.com/doc/internals/en/packet-ERR_Packet.html
	if pkgType[4] == mysql.ERR_HEADER {
		return readHandshakeError(connBuf)
	}

	return readHandshakeServerVersion(connBuf)
}

// readHandshakeServerVersion reads MySQL initial handshake message and returns the server version.
func readHandshakeServerVersion(connBuf net.Conn) (string, error) {
	dbConn := mysqlpacket.NewTLSConn(connBuf)

	handshake, err := dbConn.ReadPacket()
	if err != nil {
		return "", trace.ConnectionProblem(err, "failed to read the MySQL handshake")
	}

	if len(handshake) == 0 {
		return "", trace.Errorf("server returned empty handshake packet")
	}

	// ref: https://dev.mysql.com/doc/internals/en/connection-phase-packets.html#packet-Protocol::Handshake
	versionLength := bytes.IndexByte(handshake[1:], 0x00)
	if versionLength == -1 {
		return "", trace.Errorf("failed to read the MySQL server version")
	}

	return string(handshake[1 : 1+versionLength]), nil
}

// readHandshakeError reads and returns an error message from
func readHandshakeError(connBuf io.Reader) (string, error) {
	handshakePacket, err := ParsePacket(connBuf)
	if err != nil {
		return "", err
	}
	errPackage, ok := handshakePacket.(*Error)
	if !ok {
		return "", trace.BadParameter("expected MySQL error package, got %T", handshakePacket)
	}
	return "", trace.ConnectionProblem(errors.New("failed to fetch MySQL version"), errPackage.Error())
}

// connReader is a net.Conn wrapper with additional Peek() method.
type connReader struct {
	ctx    context.Context
	reader *bufio.Reader
	net.Conn
}

// newBufferedConn is a connReader constructor.
func newBufferedConn(ctx context.Context, conn net.Conn) connReader {
	return connReader{
		ctx:    ctx,
		reader: bufio.NewReader(conn),
		Conn:   conn,
	}
}

// Peek reads n bytes without advancing the reader.
// It's basically a wrapper around (bufio.Reader).Peek()
func (b connReader) Peek(n int) ([]byte, error) {
	if err := b.ctx.Err(); err != nil {
		return nil, err
	}
	return b.reader.Peek(n)
}

// Read returns data from underlying buffer.
func (b connReader) Read(p []byte) (int, error) {
	if err := b.ctx.Err(); err != nil {
		return 0, err
	}
	return b.reader.Read(p)
}
