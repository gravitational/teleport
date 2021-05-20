/*
Copyright 2021 Gravitational, Inc.

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

// Package mysql implements MySQL protocol support for the database access.
//
// It has the following components:
//
// * Proxy. Runs inside Teleport proxy and proxies connections from MySQL
//   clients to appropriate Teleport database services over reverse tunnel.
//
// * Engine. Runs inside Teleport database service, accepts connections coming
//   from proxy over reverse tunnel and proxies them to MySQL databases.
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
//   https://dev.mysql.com/doc/internals/en/connection-phase.html
//   https://dev.mysql.com/doc/internals/en/ssl-handshake.html
//   https://dev.mysql.com/doc/internals/en/connection-phase-packets.html
//
// The engine component plugs into the command phase and interperts all protocol
// messages flying through it:
//
//   https://dev.mysql.com/doc/internals/en/command-phase.html
//
// MySQL protocol is server-initiated meaning the first "handshake" packet
// is sent by MySQL server, as such the proxy (which acts as a server) has
// a separate listener for MySQL clients.
//
// Connection sequence diagram:
//
// mysql                   proxy
//  |                        |
//  | <--- HandshakeV10 ---  |
//  |                        |
//  | - HandshakeResponse -> |
//  |                        |
// ------- TLS upgrade --------                  engine
//  |                        |                      |
//  |                        | ----- connect -----> |
//  |                        |                      |
//  |                        |               ---- authz ----            MySQL
//  |                        |                      |                     |
//  |                        |                      | ----- connect ----> |
//  |                        |                      |                     |
//  |                        | <------- OK -------- |                     |
//  |                        |                      |                     |
//  | <-------- OK --------- |                      |                     |
//  |                        |                      |                     |
// ------------------------------ Command phase ----------------------------
//  |                        |                      |                     |
//  | ----------------------------- COM_QUERY --------------------------> |
//  | <---------------------------- Response ---------------------------- |
//  |                        |                      |                     |
//  | ----------------------------- COM_QUIT ---------------------------> |
//  |                        |                      |                     |
package mysql
