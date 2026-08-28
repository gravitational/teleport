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

import "encoding/binary"

// skipHeaderAndType skips packet header and command type, and returns rest of
// the bytes.
func skipHeaderAndType(input []byte) (unread []byte, ok bool) {
	return skipBytes(input, packetHeaderAndTypeSize)
}

// skipBytes skips n bytes from input and returns rest of the bytes.
func skipBytes(input []byte, n int) (unread []byte, ok bool) {
	if len(input) < n {
		return nil, false
	}
	return input[n:], true
}

// readByte reads one byte from input and returns rest of the bytes.
func readByte(input []byte) (unread []byte, read byte, ok bool) {
	if len(input) < 1 {
		return nil, 0x00, false
	}
	return input[1:], input[0], true
}

// readUint32 reads an uint32 from input and returns rest of the bytes.
func readUint32(input []byte) (unread []byte, read uint32, ok bool) {
	if len(input) < 4 {
		return nil, 0, false
	}
	return input[4:], binary.LittleEndian.Uint32(input[:4]), true
}

// readUint16 reads an uint16 from input and returns rest of the bytes.
func readUint16(input []byte) (unread []byte, read uint16, ok bool) {
	if len(input) < 2 {
		return nil, 0, false
	}
	return input[2:], binary.LittleEndian.Uint16(input[:2]), true
}
