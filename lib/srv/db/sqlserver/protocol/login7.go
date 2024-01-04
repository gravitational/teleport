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
	"bytes"
	"encoding/binary"
	"io"

	"github.com/gravitational/trace"
	mssql "github.com/microsoft/go-mssqldb"
)

// Login7Packet represents a Login7 packet that defines authentication rules
// between the client and the server.
//
// https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-tds/773a62b6-ee89-4c02-9e5e-344882630aac
type Login7Packet struct {
	packet   Packet
	header   Login7Header
	username string
	database string
}

// Username returns the username from the Login7 packet.
func (p *Login7Packet) Username() string {
	return p.username
}

// Database returns the database from the Login7 packet. May be empty.
func (p *Login7Packet) Database() string {
	return p.database
}

// OptionFlags1 returns the packet's first set of option flags.
func (p *Login7Packet) OptionFlags1() uint8 {
	return p.header.OptionFlags1
}

// OptionFlags2 returns the packet's second set of option flags.
func (p *Login7Packet) OptionFlags2() uint8 {
	return p.header.OptionFlags2
}

// TypeFlags returns the packet's set of type flags.
func (p *Login7Packet) TypeFlags() uint8 {
	return p.header.TypeFlags
}

// PacketSize return the packet size from the Login7 packet.
// Packet size is used by a server to negation the size of max packet length.
func (p *Login7Packet) PacketSize() uint16 {
	return uint16(p.header.PacketSize)
}

// Login7Header contains options and offset/length pairs parsed from the Login7
// packet sent by client.
//
// Note: the order of fields in the struct matters as it gets unpacked from the
// binary stream.
type Login7Header struct {
	Length            uint32
	TDSVersion        uint32
	PacketSize        uint32
	ClientProgVer     uint32
	ClientPID         uint32
	ConnectionID      uint32
	OptionFlags1      uint8
	OptionFlags2      uint8
	TypeFlags         uint8
	OptionFlags3      uint8
	ClientTimezone    int32
	ClientLCID        uint32
	IbHostName        uint16 // offset
	CchHostName       uint16 // length
	IbUserName        uint16
	CchUserName       uint16
	IbPassword        uint16
	CchPassword       uint16
	IbAppName         uint16
	CchAppName        uint16
	IbServerName      uint16
	CchServerName     uint16
	IbUnused          uint16
	CbUnused          uint16
	IbCltIntName      uint16
	CchCltIntName     uint16
	IbLanguage        uint16
	CchLanguage       uint16
	IbDatabase        uint16
	CchDatabase       uint16
	ClientID          [6]byte
	IbSSPI            uint16
	CbSSPI            uint16
	IbAtchDBFile      uint16
	CchAtchDBFile     uint16
	IbChangePassword  uint16
	CchChangePassword uint16
	CbSSPILong        uint32
}

// ReadLogin7Packet reads Login7 packet from the reader.
func ReadLogin7Packet(r io.Reader) (*Login7Packet, error) {
	pkt, err := ReadPacket(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if pkt.Type() != PacketTypeLogin7 {
		return nil, trace.BadParameter("expected Login7 packet, got: %#v", pkt)
	}

	var header Login7Header
	if err := binary.Read(bytes.NewReader(pkt.Data()), binary.LittleEndian, &header); err != nil {
		return nil, trace.Wrap(err)
	}

	username, err := readUsername(pkt, header)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	database, err := readDatabase(pkt, header)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Login7Packet{
		packet:   pkt,
		header:   header,
		username: username,
		database: database,
	}, nil
}

// errInvalidPacket is returned when Login7 package contains invalid data.
var errInvalidPacket = trace.Errorf("invalid login7 packet")

// readUsername reads username from login7 package.
func readUsername(pkt Packet, header Login7Header) (string, error) {
	if len(pkt.Data()) < int(header.IbUserName)+int(header.CchUserName)*2 {
		return "", errInvalidPacket
	}

	// Decode username and database from the packet. Offset/length are counted
	// from the beginning of entire packet data (excluding header).
	username, err := mssql.ParseUCS2String(
		pkt.Data()[header.IbUserName : header.IbUserName+header.CchUserName*2])
	if err != nil {
		return "", trace.Wrap(err)
	}
	return username, nil
}

// readDatabase reads database name from login7 package.
func readDatabase(pkt Packet, header Login7Header) (string, error) {
	if len(pkt.Data()) < int(header.IbDatabase)+int(header.CchDatabase)*2 {
		return "", errInvalidPacket
	}

	database, err := mssql.ParseUCS2String(
		pkt.Data()[header.IbDatabase : header.IbDatabase+header.CchDatabase*2])
	if err != nil {
		return "", trace.Wrap(err)
	}
	return database, nil
}
