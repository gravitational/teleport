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

const (
	// PacketTypeSQLBatch is the SQLBatch packet type.
	PacketTypeSQLBatch uint8 = 0x01
	// PacketTypeRPCRequest is the RPCRequest packet type.
	PacketTypeRPCRequest uint8 = 0x03

	// PacketTypeResponse is the packet type for server response messages.
	PacketTypeResponse uint8 = 0x04
	// PacketTypeLogin7 is the Login7 packet type.
	PacketTypeLogin7 uint8 = 0x10
	// PacketTypePreLogin is the Pre-Login packet type.
	PacketTypePreLogin uint8 = 0x12

	// packetHeaderSize is the size of the protocol packet header.
	packetHeaderSize = 8

	// PacketStatusLast indicates that the packet is the last in the request.
	// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/ce398f9a-7d47-4ede-8f36-9dd6fc21ca43
	PacketStatusLast uint8 = 0x01

	preLoginOptionVersion    = 0x00
	preLoginOptionEncryption = 0x01
	preLoginOptionInstance   = 0x02
	preLoginOptionThreadID   = 0x03
	preLoginOptionMARS       = 0x04

	// preLoginEncryptionNotSupported is a Pre-Login option indicating that
	// server does not accept TLS connection (clients connect through local
	// proxy's TLS tunnel).
	preLoginEncryptionNotSupported = 0x02

	// errorClassSecurity is the SQL Server error class representing security
	// related errors such as access denied.
	errorClassSecurity uint8 = 14
	// errorNumber is the error number used for all Teleport-returned errors.
	// Numbers < 20001 are reserved by SQL Server.
	errorNumber = 28353
)

// preLoginOptions are getting returned to the client during Pre-Login handshake.
//
// SQL Server clients always connect to the local proxy and connections come
// through TLS tunnel.
var preLoginOptions = map[uint8][]byte{
	preLoginOptionVersion:    []uint8{0x0f, 0x00, 0x07, 0xd0, 0x00, 0x00},
	preLoginOptionEncryption: {preLoginEncryptionNotSupported},
	preLoginOptionInstance:   {0x00},
	preLoginOptionThreadID:   {},
	preLoginOptionMARS:       {0x00},
}

const (
	// procIDSwitchRPCRequest is a magic value defined in:
	// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/619c43b6-9495-4a58-9e49-a4950db245b3
	// as  "ProcIDSwitch     =   %xFF %xFF" used to distinguish user custom user procedure.
	procIDSwitchRPCRequest = 0xFFFF
)
