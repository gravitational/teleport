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

// schemaNamePacket is a common packet format that the packet type is followed
// by the schema name.
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
// COM_INIT_DB is used to specify the default schema for the connection. For
// example, "USE <schema name>" from "mysql" client sends COM_INIT_DB command
// with the schema name.
//
// https://dev.mysql.com/doc/internals/en/com-init-db.html
// https://mariadb.com/kb/en/com_init_db/
type InitDB struct {
	schemaNamePacket
}

// CreateDB represents the COM_CREATE_DB command.
//
// https://dev.mysql.com/doc/internals/en/com-create-db.html
// https://mariadb.com/kb/en/com_create_db/
//
// COM_CREATE_DB creates a schema. COM_CREATE_DB is deprecated in both MySQL
// and MariaDB.
type CreateDB struct {
	schemaNamePacket
}

// DropDB represents the COM_DROP_DB command.
//
// https://dev.mysql.com/doc/internals/en/com-drop-db.html
// https://mariadb.com/kb/en/com_drop_db/
//
// COM_DROP_DB drops a schema. COM_DROP_DB is deprecated in both MySQL and
// MariaDB.
type DropDB struct {
	schemaNamePacket
}

// ShutDown represents the COM_SHUTDOWN command.
//
// https://dev.mysql.com/doc/internals/en/com-shutdown.html
// https://mariadb.com/kb/en/com_shutdown/
//
// COM_SHUTDOWN is used to shut down the MySQL server. COM_SHUTDOWN requires
// SHUTDOWN privileges. COM_SHUTDOWN is deprecated as of MySQL 5.7.9.
type ShutDown struct {
	packet
}

// ProcessKill represents the COM_PROCESS_KILL command.
//
// https://dev.mysql.com/doc/internals/en/com-process-kill.html
// https://mariadb.com/kb/en/com_process_kill/
//
// COM_PROCESS_KILL asks the server to terminate a connection. COM_PROCESS_KILL
// is deprecated as of MySQL 5.7.11.
type ProcessKill struct {
	packet

	// processID is the process ID of a connection.
	processID uint32
}

// ProcessID returns the process ID of a connection.
func (p *ProcessKill) ProcessID() uint32 {
	return p.processID
}

// Debug represents the COM_DEBUG command.
//
// https://dev.mysql.com/doc/internals/en/com-debug.html
// https://mariadb.com/kb/en/com_debug/
//
// COM_DEBUG forces the server to dump debug information to stdout. COM_DEBUG
// requires SUPER privileges.
type Debug struct {
	packet
}

// Refresh represents the COM_REFRESH command.
//
// https://dev.mysql.com/doc/internals/en/com-refresh.html
//
// COM_REFRESH calls REFRESH or FLUSH statements. COM_REFRESH is deprecated as
// of MySQL 5.7.11.
type Refresh struct {
	packet

	// subcommand is the string representation of the subcommand.
	subcommand string
}

// Subcommand returns the string representation of the subcommand.
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
	idx := bytes.IndexByte(packetBytes[packetHeaderAndTypeSize:], 0x00)
	if idx < 0 {
		return nil, trace.BadParameter("failed to parse COM_CHANGE_USER packet: %v", packetBytes)
	}

	return &ChangeUser{
		packet: packet{bytes: packetBytes},
		user:   string(packetBytes[packetHeaderAndTypeSize : packetHeaderAndTypeSize+idx]),
	}, nil
}

// parseSchemaNamePacket parses packet bytes and returns a schemaNamePacket if
// successful.
func parseSchemaNamePacket(packetBytes []byte) (schemaNamePacket, bool) {
	unread, ok := skipHeaderAndType(packetBytes)
	if !ok {
		return schemaNamePacket{}, false
	}

	return schemaNamePacket{
		packet:     packet{bytes: packetBytes},
		schemaName: string(unread),
	}, true
}

// parseInitDBPacket parses packet bytes and returns a Packet if successful.
func parseInitDBPacket(packetBytes []byte) (Packet, error) {
	parent, ok := parseSchemaNamePacket(packetBytes)
	if !ok {
		return nil, trace.BadParameter("failed to parse COM_INIT_DB packet: %v", packetBytes)
	}

	return &InitDB{
		schemaNamePacket: parent,
	}, nil
}

// parseCreateDBPacket parses packet bytes and returns a Packet if successful.
func parseCreateDBPacket(packetBytes []byte) (Packet, error) {
	parent, ok := parseSchemaNamePacket(packetBytes)
	if !ok {
		return nil, trace.BadParameter("failed to parse COM_CREATE_DB packet: %v", packetBytes)
	}

	return &CreateDB{
		schemaNamePacket: parent,
	}, nil
}

// parseDropDBPacket parses packet bytes and returns a Packet if successful.
func parseDropDBPacket(packetBytes []byte) (Packet, error) {
	parent, ok := parseSchemaNamePacket(packetBytes)
	if !ok {
		return nil, trace.BadParameter("failed to parse COM_DROP_DB packet: %v", packetBytes)
	}

	return &DropDB{
		schemaNamePacket: parent,
	}, nil
}

// parseShutDownPacket parses packet bytes and returns a Packet if successful.
func parseShutDownPacket(packetBytes []byte) (Packet, error) {
	return &ShutDown{
		packet: packet{bytes: packetBytes},
	}, nil
}

// parseProcessKillPacket parses packet bytes and returns a Packet if successful.
func parseProcessKillPacket(packetBytes []byte) (Packet, error) {
	unread, ok := skipHeaderAndType(packetBytes)
	if !ok {
		return nil, trace.BadParameter("failed to parse COM_PROCESS_KILL packet: %v", packetBytes)
	}

	_, processID, ok := readUint32(unread)
	if !ok {
		return nil, trace.BadParameter("failed to parse COM_PROCESS_KILL packet: %v", packetBytes)
	}

	return &ProcessKill{
		packet:    packet{bytes: packetBytes},
		processID: processID,
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
