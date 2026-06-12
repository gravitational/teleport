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
	"github.com/gravitational/trace"
)

// OK represents the OK packet.
//
// https://dev.mysql.com/doc/internals/en/packet-OK_Packet.html
// https://mariadb.com/kb/en/ok_packet/
type OK struct {
	packet
}

// Error represents the ERR packet.
//
// https://dev.mysql.com/doc/internals/en/packet-ERR_Packet.html
// https://mariadb.com/kb/en/err_packet/
type Error struct {
	packet

	// Message is the error Message
	Message string
	// Code is the error Code.
	Code uint16
}

// Error returns the error message.
func (p *Error) Error() string {
	return p.Message
}

// parseOKPacket parses packet bytes and returns a Packet if successful.
func parseOKPacket(packetBytes []byte) (Packet, error) {
	return &OK{
		packet: packet{bytes: packetBytes},
	}, nil
}

// parseErrorPacket parses packet bytes and returns a Packet if successful.
func parseErrorPacket(packetBytes []byte) (Packet, error) {
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

	// ignore unread bytes and "ok", we already checked len of bytes.
	_, code, _ := readUint16(packetBytes[packetHeaderAndTypeSize:])

	return &Error{
		packet:  packet{bytes: packetBytes},
		Message: string(packetBytes[minLen:]),
		Code:    code,
	}, nil
}
