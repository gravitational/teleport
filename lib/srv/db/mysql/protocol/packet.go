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

package protocol

import (
	"io"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/gravitational/trace"
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

// ParsePacket reads a protocol packet from the connection and returns it
// in a parsed form. See ReadPacket below for the packet structure.
func ParsePacket(conn io.Reader) (Packet, error) {
	packetBytes, packetType, err := ReadPacket(conn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if int(packetType) < len(packetParsersByType) && packetParsersByType[packetType] != nil {
		packet, err := packetParsersByType[packetType](packetBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return packet, nil
	}

	return &Generic{
		packet: packet{bytes: packetBytes},
	}, nil
}

// ReadPacket reads a protocol packet from the connection.
//
// MySQL wire protocol packet has the following structure:
//
//	 4-byte
//	 header      payload
//	 ________    _________ ...
//	|        |  |
//
// xx xx xx xx xx xx xx xx ...
//
//	|_____|  |  |
//	payload  |  message
//	length   |  type
//	         |
//	         sequence
//	         number
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

// packetParsersByType is a slice of packet parser functions by packet type.
var packetParsersByType = []func([]byte) (Packet, error){
	// Server responses.
	mysql.OK_HEADER:  parseOKPacket,
	mysql.ERR_HEADER: parseErrorPacket,

	// Text protocol commands.
	mysql.COM_QUERY:        parseQueryPacket,
	mysql.COM_QUIT:         parseQuitPacket,
	mysql.COM_CHANGE_USER:  parseChangeUserPacket,
	mysql.COM_INIT_DB:      parseInitDBPacket,
	mysql.COM_CREATE_DB:    parseCreateDBPacket,
	mysql.COM_DROP_DB:      parseDropDBPacket,
	mysql.COM_SHUTDOWN:     parseShutDownPacket,
	mysql.COM_PROCESS_KILL: parseProcessKillPacket,
	mysql.COM_DEBUG:        parseDebugPacket,
	mysql.COM_REFRESH:      parseRefreshPacket,

	// Prepared statement commands.
	mysql.COM_STMT_PREPARE:         parseStatementPreparePacket,
	mysql.COM_STMT_SEND_LONG_DATA:  parseStatementSendLongDataPacket,
	mysql.COM_STMT_EXECUTE:         parseStatementExecutePacket,
	mysql.COM_STMT_CLOSE:           parseStatementClosePacket,
	mysql.COM_STMT_RESET:           parseStatementResetPacket,
	mysql.COM_STMT_FETCH:           parseStatementFetchPacket,
	packetTypeStatementBulkExecute: parseStatementBulkExecutePacket,
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
