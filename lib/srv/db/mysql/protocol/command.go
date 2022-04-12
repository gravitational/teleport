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

// schemaNamePacket is a common packet that the schema name is followed after
// the packet type.
type schemaNamePacket struct {
	packet

	// schemaName is the schema name.
	schemaName string
}

// SchemaName returns the schema name.
func (p *schemaNamePacket) SchemaName() string {
	return p.schemaName
}

// InitDB represents the COM_INIT_DB command.
//
// https://dev.mysql.com/doc/internals/en/com-init-db.html
// https://mariadb.com/kb/en/com_init_db/
type InitDB struct {
	schemaNamePacket
}

// CreateDB represents the COM_CREATE_DB command.
//
// https://dev.mysql.com/doc/internals/en/com-create-db.html
//
// https://mariadb.com/kb/en/com_create_db/
// Deprecated in MariaDB.
type CreateDB struct {
	schemaNamePacket
}

// DropDB represents the COM_DROP_DB command.
//
// https://dev.mysql.com/doc/internals/en/com-drop-db.html
//
// https://mariadb.com/kb/en/com_drop_db/
// Deprecated in MariaDB.
type DropDB struct {
	schemaNamePacket
}

// Shutdown represents the COM_SHUTDOWN command.
//
// https://dev.mysql.com/doc/internals/en/com-shutdown.html
// Deprecated as of MySQL 5.7.9. Support end of life is October, 2023.
//
// https://mariadb.com/kb/en/com_shutdown/
type Shutdown struct {
	packet
}

// ProcessKill represents the COM_PROCESS_KILL command.
//
// https://dev.mysql.com/doc/internals/en/com-process-kill.html
// Deprecated as of MySQL 5.7.11. Support end of life is October, 2023.
//
// https://mariadb.com/kb/en/com_process_kill/
type ProcessKill struct {
	packet

	// connectionID is the connection ID
	connectionID uint32
}

// ConnectionID returns the connection ID.
func (p *ProcessKill) ConnectionID() uint32 {
	return p.connectionID
}

// Debug represents the COM_DEBUG command.
//
// https://dev.mysql.com/doc/internals/en/com-debug.html
// https://mariadb.com/kb/en/com_debug/
type Debug struct {
	packet
}

// Refresh represents the COM_REFRESH command.
//
// https://dev.mysql.com/doc/internals/en/com-refresh.html
// Deprecated as of MySQL 5.7.11. Support end of life is October, 2023.
type Refresh struct {
	packet

	// subcommand is the string representation of the subcomand.
	subcommand string
}

// Subcommand returns the string representation of the subcomand.
func (p *Refresh) Subcommand() string {
	return p.subcommand
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
	_, user, ok := readNullTerminatedString(packetBytes[packetHeaderAndTypeSize:])
	if !ok {
		return nil, trace.BadParameter("failed to parse COM_CHANGE_USER packet: %v", packetBytes)
	}

	return &ChangeUser{
		packet: packet{bytes: packetBytes},
		user:   user,
	}, nil
}

// parseSchameNamePacket parses packet bytes and returns a schemaNamePacket if
// successful.
func parseSchameNamePacket(packetBytes []byte) (schemaNamePacket, bool) {
	unread, ok := skipHeaderAndType(packetBytes)
	if !ok {
		return schemaNamePacket{}, false
	}

	_, schemaName, ok := readNullTerminatedString(unread)
	if !ok {
		return schemaNamePacket{}, false
	}

	return schemaNamePacket{
		packet:     packet{bytes: packetBytes},
		schemaName: schemaName,
	}, true
}

// parseInitDBPacket parses packet bytes and returns a Packet if successful.
func parseInitDBPacket(packetBytes []byte) (Packet, error) {
	parent, ok := parseSchameNamePacket(packetBytes)
	if !ok {
		return nil, trace.BadParameter("failed to parse COM_INIT_DB packet: %v", packetBytes)
	}

	return &InitDB{
		schemaNamePacket: parent,
	}, nil
}

// parseCreateDBPacket parses packet bytes and returns a Packet if successful.
func parseCreateDBPacket(packetBytes []byte) (Packet, error) {
	parent, ok := parseSchameNamePacket(packetBytes)
	if !ok {
		return nil, trace.BadParameter("failed to parse COM_CREATE_DB packet: %v", packetBytes)
	}

	return &CreateDB{
		schemaNamePacket: parent,
	}, nil
}

// parseDropDBPacket parses packet bytes and returns a Packet if successful.
func parseDropDBPacket(packetBytes []byte) (Packet, error) {
	parent, ok := parseSchameNamePacket(packetBytes)
	if !ok {
		return nil, trace.BadParameter("failed to parse COM_CREATE_DB packet: %v", packetBytes)
	}

	return &DropDB{
		schemaNamePacket: parent,
	}, nil
}

// parseShutdownPacket parses packet bytes and returns a Packet if successful.
func parseShutdownPacket(packetBytes []byte) (Packet, error) {
	return &Shutdown{
		packet: packet{bytes: packetBytes},
	}, nil
}

// parseProcessKillPacket parses packet bytes and returns a Packet if successful.
func parseProcessKillPacket(packetBytes []byte) (Packet, error) {
	unread, ok := skipHeaderAndType(packetBytes)
	if !ok {
		return nil, trace.BadParameter("failed to parse COM_PROCESS_KILL packet: %v", packetBytes)
	}

	_, connectionID, ok := readUint32(unread)
	if !ok {
		return nil, trace.BadParameter("failed to parse COM_PROCESS_KILL packet: %v", packetBytes)
	}

	return &ProcessKill{
		packet:       packet{bytes: packetBytes},
		connectionID: connectionID,
	}, nil
}

// parseDebugPacket parses packet bytes and returns a Packet if successful.
func parseDebugPacket(packetBytes []byte) (Packet, error) {
	return &Debug{
		packet: packet{bytes: packetBytes},
	}, nil
}

// parseRefreshPacket parses packet bytes and returns a Packet if successful.
func parseRefreshPacket(packetBytes []byte) (Packet, error) {
	unread, ok := skipHeaderAndType(packetBytes)
	if !ok {
		return nil, trace.BadParameter("failed to parse COM_REFRESH packet: %v", packetBytes)
	}

	_, subcommandByte, ok := readByte(unread)
	if !ok {
		return nil, trace.BadParameter("failed to parse COM_REFRESH packet: %v", packetBytes)
	}

	var subcommand string
	switch subcommandByte {
	case 0x01:
		subcommand = "REFRESH_GRANT"
	case 0x02:
		subcommand = "REFRESH_LOG"
	case 0x04:
		subcommand = "REFRESH_TABLES"
	case 0x08:
		subcommand = "REFRESH_HOSTS"
	case 0x10:
		subcommand = "REFRESH_STATUS"
	case 0x20:
		subcommand = "REFRESH_THREADS"
	case 0x40:
		subcommand = "REFRESH_SLAVE"
	case 0x80:
		subcommand = "REFRESH_MASTER"
	default:
		return nil, trace.BadParameter("failed to parse COM_REFRESH packet: %v", packetBytes)
	}

	return &Refresh{
		packet:     packet{bytes: packetBytes},
		subcommand: subcommand,
	}, nil
}
