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

// Package protocol implements parts of MySQL wire protocol which are needed
// for the service to be able to interpret the protocol messages but are not
// readily available in the convenient form in the vendored MySQL library.
//
// For example, reading protocol packets from connections, parsing them and
// writing them to connections.
//
// The following resources are helpful to understand protocol details.
//
// Packet structure:
//
//	https://dev.mysql.com/doc/internals/en/mysql-packet.html
//
// Generic response packets:
//
//	https://dev.mysql.com/doc/internals/en/generic-response-packets.html
//
// Packets sent in the command phase:
//
//	https://dev.mysql.com/doc/internals/en/command-phase.html
package protocol
