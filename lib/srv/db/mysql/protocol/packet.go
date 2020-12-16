/*
Copyright 2020 Gravitational, Inc.

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
	"io"
	"net"

	"github.com/gravitational/trace"
)

// ReadPacket reads a protocol packet from the provided connection.
//
// https://dev.mysql.com/doc/internals/en/mysql-packet.html
func ReadPacket(conn net.Conn) ([]byte, error) {
	// MySQL protocol packet header is 4 bytes.
	header := []byte{0, 0, 0, 0}
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	// First 3 bytes is the payload length.
	payloadLength := int(uint32(header[0]) | uint32(header[1])<<8 | uint32(header[2])<<16)
	// Read full packet body.
	payload := make([]byte, payloadLength)
	n, err := io.ReadFull(conn, payload)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return append(header, payload[0:n]...), nil
}

// WritePacket writes the provided protocol packet to the connection.
func WritePacket(pkt []byte, conn net.Conn) (int, error) {
	n, err := conn.Write(pkt)
	if err != nil {
		return 0, trace.ConvertSystemError(err)
	}
	return n, nil
}
