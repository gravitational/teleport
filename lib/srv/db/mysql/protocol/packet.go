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

// ChangeUser represents the COM_CHANGE_USER command.
type ChangeUser struct {
	packet
	// user is the requested user.
	user string
}

// User returns the requested user.
func (p *ChangeUser) User() string {
	return p.user
}

// ParsePacket reads a protocol packet from the connection and returns it
// in a parsed form. See ReadPacket below for the packet structure.
func ParsePacket(conn io.Reader) (Packet, error) {
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
		minLen := packetHeaderSize + packetTypeSize + 2 // 4-byte header + 1-byte type + 2-byte error code
		if len(packetBytes) > minLen && packetBytes[minLen] == '#' {
			minLen += 6 // 1-byte marker '#' + 5-byte state
		}
		// Be a bit paranoid and make sure the packet is not truncated.
		if len(packetBytes) < minLen {
			return nil, trace.BadParameter("failed to parse ERR packet: %v", packetBytes)
		}
		return &Error{packet: packet, message: string(packetBytes[minLen:])}, nil

	case mysql.COM_QUERY:
		// Be a bit paranoid and make sure the packet is not truncated.
		if len(packetBytes) < packetHeaderAndTypeSize {
			return nil, trace.BadParameter("failed to parse COM_QUERY packet: %v", packetBytes)
		}
		// 4-byte packet header + 1-byte payload header, then query text.
		return &Query{packet: packet, query: string(packetBytes[packetHeaderAndTypeSize:])}, nil

	case mysql.COM_QUIT:
		return &Quit{packet: packet}, nil

	case mysql.COM_CHANGE_USER:
		if len(packetBytes) < packetHeaderAndTypeSize {
			return nil, trace.BadParameter("failed to parse COM_CHANGE_USER packet: %v", packetBytes)
		}
		// User is the first null-terminated string in the payload:
		// https://dev.mysql.com/doc/internals/en/com-change-user.html#packet-COM_CHANGE_USER
		idx := bytes.IndexByte(packetBytes[packetHeaderAndTypeSize:], 0x00)
		if idx < 0 {
			return nil, trace.BadParameter("failed to parse COM_CHANGE_USER packet: %v", packetBytes)
		}
		return &ChangeUser{packet: packet, user: string(packetBytes[packetHeaderAndTypeSize : packetHeaderAndTypeSize+idx])}, nil

	case mysql.COM_STMT_PREPARE:
		packet, ok := parseStatementPreparePacket(packet)
		if !ok {
			return nil, trace.BadParameter("failed to parse COM_STMT_PREPARE packet: %v", packetBytes)
		}
		return packet, nil

	case mysql.COM_STMT_SEND_LONG_DATA:
		packet, ok := parseStatementSendLongDataPacket(packet)
		if !ok {
			return nil, trace.BadParameter("failed to parse COM_STMT_SEND_LONG_DATA packet: %v", packetBytes)
		}
		return packet, nil

	case mysql.COM_STMT_EXECUTE:
		packet, ok := parseStatementExecutePacket(packet)
		if !ok {
			return nil, trace.BadParameter("failed to parse COM_STMT_EXECUTE packet: %v", packetBytes)
		}
		return packet, nil

	case mysql.COM_STMT_CLOSE:
		packet, ok := parseStatementClosePacket(packet)
		if !ok {
			return nil, trace.BadParameter("failed to parse COM_STMT_CLOSE packet: %v", packetBytes)
		}
		return packet, nil

	case mysql.COM_STMT_RESET:
		packet, ok := parseStatementResetPacket(packet)
		if !ok {
			return nil, trace.BadParameter("failed to parse COM_STMT_RESET packet: %v", packetBytes)
		}
		return packet, nil

	case mysql.COM_STMT_FETCH:
		packet, ok := parseStatementFetchPacket(packet)
		if !ok {
			return nil, trace.BadParameter("failed to parse COM_STMT_FETCH packet: %v", packetBytes)
		}
		return packet, nil

	case packetTypeStatementBulkExecute:
		packet, ok := parseStatementBulkExecutePacket(packet)
		if !ok {
			return nil, trace.BadParameter("failed to parse COM_STMT_BULK_EXECUTE packet: %v", packetBytes)
		}
		return packet, nil
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
func ReadPacket(conn io.Reader) (pkt []byte, pktType byte, err error) {
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
func WritePacket(pkt []byte, conn io.Writer) (int, error) {
	n, err := conn.Write(pkt)
	if err != nil {
		return 0, trace.ConvertSystemError(err)
	}
	return n, nil
}

const (
	// packetHeaderSize is the size of the packet header.
	packetHeaderSize = 4

	// packetTypeSize is the size of the command type.
	packetTypeSize = 1

	// packetHeaderAndTypeSize is the combined size of the packet header and
	// type.
	packetHeaderAndTypeSize = packetHeaderSize + packetTypeSize
)

const (
	// packetTypeStatementBulkExecute is a MariaDB specific packet type for
	// COM_STMT_BULK_EXECUTE packets.
	//
	// https://mariadb.com/kb/en/com_stmt_bulk_execute/
	packetTypeStatementBulkExecute = 0xfa
)
