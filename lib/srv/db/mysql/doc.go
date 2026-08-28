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

// Package mysql implements MySQL protocol support for the database access.
//
// It has the following components:
//
//   - Proxy. Runs inside Teleport proxy and proxies connections from MySQL
//     clients to appropriate Teleport database services over reverse tunnel.
//
//   - Engine. Runs inside Teleport database service, accepts connections coming
//     from proxy over reverse tunnel and proxies them to MySQL databases.
//
// Protocol
// --------
//
// MySQL protocol consists of two phases, connection phase and command phase.
//
// The proxy component implements the connection phase in order to be able to
// get client's x509 certificate which contains all the auth and routing
// information in it:
//
//	https://dev.mysql.com/doc/internals/en/connection-phase.html
//	https://dev.mysql.com/doc/internals/en/ssl-handshake.html
//	https://dev.mysql.com/doc/internals/en/connection-phase-packets.html
//
// The engine component plugs into the command phase and interperts all protocol
// messages flying through it:
//
//	https://dev.mysql.com/doc/internals/en/command-phase.html
//
// MySQL protocol is server-initiated meaning the first "handshake" packet
// is sent by MySQL server, as such the proxy (which acts as a server) has
// a separate listener for MySQL clients.
//
// Connection sequence diagram:
//
// mysql                   proxy
//
//	|                        |
//	| <--- HandshakeV10 ---  |
//	|                        |
//	| - HandshakeResponse -> |
//	|                        |
//
// ------- TLS upgrade --------                  engine
//
//	|                        |                      |
//	|                        | ----- connect -----> |
//	|                        |                      |
//	|                        |               ---- authz ----            MySQL
//	|                        |                      |                     |
//	|                        |                      | ----- connect ----> |
//	|                        |                      |                     |
//	|                        | <------- OK -------- |                     |
//	|                        |                      |                     |
//	| <-------- OK --------- |                      |                     |
//	|                        |                      |                     |
//
// ------------------------------ Command phase ----------------------------
//
//	|                        |                      |                     |
//	| ----------------------------- COM_QUERY --------------------------> |
//	| <---------------------------- Response ---------------------------- |
//	|                        |                      |                     |
//	| ----------------------------- COM_QUIT ---------------------------> |
//	|                        |                      |                     |
package mysql
