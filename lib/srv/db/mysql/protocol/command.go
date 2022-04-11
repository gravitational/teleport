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
	"bytes"

	"github.com/gravitational/trace"
)

// Query represents the COM_QUERY command.
//
// https://dev.mysql.com/doc/internals/en/com-query.html
// https://mariadb.com/kb/en/com_query/
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
// https://mariadb.com/kb/en/com_quit/
type Quit struct {
	packet
}

// ChangeUser represents the COM_CHANGE_USER command.
//
// https://dev.mysql.com/doc/internals/en/com-change-user.html
// https://mariadb.com/kb/en/com_change_user/
type ChangeUser struct {
	packet

	// user is the requested user.
	user string
}

// User returns the requested user.
func (p *ChangeUser) User() string {
	return p.user
}

// parseQueryPacket parses packet bytes and returns a Packet if successful.
func parseQueryPacket(packetBytes []byte) (Packet, error) {
	// Be a bit paranoid and make sure the packet is not truncated.
	if len(packetBytes) < packetHeaderAndTypeSize {
		return nil, trace.BadParameter("failed to parse COM_QUERY packet: %v", packetBytes)
	}

	// 4-byte packet header + 1-byte payload header, then query text.
	return &Query{
		packet: packet{bytes: packetBytes},
		query:  string(packetBytes[packetHeaderAndTypeSize:]),
	}, nil
}

// parseQuitPacket parses packet bytes and returns a Packet if successful.
func parseQuitPacket(packetBytes []byte) (Packet, error) {
	return &Quit{
		packet: packet{bytes: packetBytes},
	}, nil
}

// parseChangeUserPacket parses packet bytes and returns a Packet if
// successful.
func parseChangeUserPacket(packetBytes []byte) (Packet, error) {
	if len(packetBytes) < packetHeaderAndTypeSize {
		return nil, trace.BadParameter("failed to parse COM_CHANGE_USER packet: %v", packetBytes)
	}

	// User is the first null-terminated string in the payload:
	// https://dev.mysql.com/doc/internals/en/com-change-user.html#packet-COM_CHANGE_USER
	idx := bytes.IndexByte(packetBytes[packetHeaderAndTypeSize:], 0x00)
	if idx == -1 {
		return nil, trace.BadParameter("failed to parse COM_CHANGE_USER packet: %v", packetBytes)
	}

	return &ChangeUser{
		packet: packet{bytes: packetBytes},
		user:   string(packetBytes[packetHeaderAndTypeSize : packetHeaderAndTypeSize+idx]),
	}, nil
}
