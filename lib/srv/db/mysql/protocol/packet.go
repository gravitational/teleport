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

package protocol

import (
	"bytes"
	"io"
	"net"

	"github.com/gravitational/trace"
	"github.com/siddontang/go-mysql/mysql"
)

// Packet is the common interface for MySQL wire protocol packets.
type Packet interface {
	// Bytes returns the packet as raw bytes.
	Bytes() []byte
}

// packet is embedded in all packets to provide common methods.
type packet struct {
	// bytes is the entire packet bytes.
	bytes []byte
}

// Bytes returns the packet as raw bytes.
func (p *packet) Bytes() []byte {
	return p.bytes
}

// Generic represents a generic packet other than the ones recognized below.
type Generic struct {
	packet
}

// OK represents the OK packet.
//
// https://dev.mysql.com/doc/internals/en/packet-OK_Packet.html
type OK struct {
	packet
}

// Error represents the ERR packet.
//
// https://dev.mysql.com/doc/internals/en/packet-ERR_Packet.html
type Error struct {
	packet
	// message is the error message
	message string
}

// Error returns the error message.
func (p *Error) Error() string {
	return p.message
}

// Query represents the COM_QUERY command.
//
// https://dev.mysql.com/doc/internals/en/com-query.html
type Query struct {
	packet
	// query is the query text.
	query string
}

// Query returns the query text.
func (p *Query) Query() string {
	return p.query
}

// Quit represents the COM_QUIT command.
//
// https://dev.mysql.com/doc/internals/en/com-quit.html
type Quit struct {
	packet
}

// ParsePacket reads a protocol packet from the connection and returns it
// in a parsed form. See ReadPacket below for the packet structure.
func ParsePacket(conn net.Conn) (Packet, error) {
	packetBytes, packetType, err := ReadPacket(conn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	packet := packet{packetBytes}

	switch packetType {
	case mysql.OK_HEADER:
		return &OK{packet: packet}, nil

	case mysql.ERR_HEADER:
		// Figure out where in the packet the error message is.
		//
		// Depending on the protocol version, the packet may include additional
		// fields. In protocol version 4.1 it includes '#' marker:
		//
		// https://dev.mysql.com/doc/internals/en/packet-ERR_Packet.html
		minLen := 7 // 4-byte header + 3-byte payload before message
		if bytes.Contains(packetBytes, []byte("#")) {
			minLen = 13 // 4-byte header + 9-byte payload before message
		}
		// Be a bit paranoid and make sure the packet is not truncated.
		if len(packetBytes) < minLen {
			return nil, trace.BadParameter("failed to parse ERR packet: %v", packetBytes)
		}
		return &Error{packet: packet, message: string(packetBytes[minLen:])}, nil

	case mysql.COM_QUERY:
		// Be a bit paranoid and make sure the packet is not truncated.
		if len(packetBytes) < 5 {
			return nil, trace.BadParameter("failed to parse COM_QUERY packet: %v", packetBytes)
		}
		// 4-byte packet header + 1-byte payload header, then query text.
		return &Query{packet: packet, query: string(packetBytes[5:])}, nil

	case mysql.COM_QUIT:
		return &Quit{packet: packet}, nil
	}

	return &Generic{packet: packet}, nil
}

// ReadPacket reads a protocol packet from the connection.
//
// MySQL wire protocol packet has the following structure:
//
//   4-byte
//   header      payload
//   ________    _________ ...
//  |        |  |
// xx xx xx xx xx xx xx xx ...
//  |_____|  |  |
//  payload  |  message
//  length   |  type
//           |
//           sequence
//           number
//
// https://dev.mysql.com/doc/internals/en/mysql-packet.html
func ReadPacket(conn net.Conn) (pkt []byte, pktType byte, err error) {
	// Read 4-byte packet header.
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return nil, 0, trace.ConvertSystemError(err)
	}

	// First 3 header bytes is the payload length, the 4th is the sequence
	// number which we have no use for.
	payloadLen := int(uint32(header[0]) | uint32(header[1])<<8 | uint32(header[2])<<16)
	if payloadLen == 0 {
		return header[:], 0, nil
	}

	// Read the packet payload.
	// TODO(r0mant): Couple of improvements could be made here:
	// * Reuse buffers instead of allocating everything from scratch every time.
	// * Max payload size is 16Mb, support reading larger packets.
	payload := make([]byte, payloadLen)
	n, err := io.ReadFull(conn, payload)
	if err != nil {
		return nil, 0, trace.ConvertSystemError(err)
	}

	// First payload byte typically indicates the command type (query, quit,
	// etc) so return it separately.
	return append(header[:], payload[0:n]...), payload[0], nil
}

// WritePacket writes the provided protocol packet to the connection.
func WritePacket(pkt []byte, conn net.Conn) (int, error) {
	n, err := conn.Write(pkt)
	if err != nil {
		return 0, trace.ConvertSystemError(err)
	}
	return n, nil
}
