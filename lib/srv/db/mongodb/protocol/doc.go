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

// Package protocol implements reading/writing MongoDB wire protocol messages
// from/to client/server and converting them into parsed data structures.
//
// The official Go MongoDB driver provides low-level wire message parsing
// primitives this package is built on top of:
//
// https://pkg.go.dev/go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage
//
// MongoDB wire protocol documentation:
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/
//
// We implement a subset of protocol messages: OP_MSG, OP_QUERY and OP_REPLY.
//
// Package layout:
//
//   - message.go: Defines wire message common interface and provides methods for
//     reading wire messages from client/server connections.
//
//   - opmsg.go: Contains marshal/unmarshal for OP_MSG - extensible message that
//     MongoDB 3.6 and higher use for all commands.
//
//   - opquery.go: Contains marshal/unmarshal for OP_QUERY - a legacy command,
//     still used for some operations (e.g. first "isMaster" handshake message).
//
//   - opreply.go: Contains marshal/unmarshal for OP_REPLY - a reply message sent
//     by a database to an OP_QUERY command.
//
//   - errors.go: Provides methods for sending errors in wire message to client
//     connections.
package protocol
