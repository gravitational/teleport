/*
Copyright 2020 Gravitational, Inc.

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

// Package postgres implements components of the database access subsystem
// that proxy connections between Postgres clients (like, psql or pgAdmin)
// and Postgres database servers with full protocol awareness.
//
// It understands Postgres wire protocol version 3 which is supported by
// Postgres 7.4 and later:
//
// https://www.postgresql.org/docs/13/protocol-flow.html
// https://www.postgresql.org/docs/13/protocol-message-formats.html
//
// The package provides the following main types:
//
// * Proxy. Runs inside Teleport proxy and proxies connections from Postgres
//   clients to appropriate database servers over reverse tunnel.
//
// * Engine. Runs inside Teleport database service, accepts connections
//   coming from proxy over reversetunnel and proxies them to databases.
//
// * TestServer. Fake Postgres server that implements a small part of its
//   wire protocol, used in functional tests.
//
// Protocol
// --------
//
// When connecting to a database server (or a Teleport proxy in our case),
// Postgres clients start on a plain connection to check whether the server
// supports TLS and then upgrade the connection.
//
// Because of that, the proxy implements a part of the startup protocol that
// indicates TLS support to the client to get it to send a client certificate
// and extracts the identity/routing information from it.
//
// After that proxy hands off the connection to an appropriate database server
// based on the extracted routing info over reverse tunnel.
//
// The database server in turn connects to the database and starts relaying
// messages between the database and the client.
//
// The sequence diagram roughly looks like this:
//
// psql                   proxy
//  |                       |
//  | ---- SSLRequest ----> |
//  |                       |
//  | <------  'S' -------- |
//  |                       |
//  | -- StartupMessage --> |                     engine
//  |                       |                       |
//  |                       | -- StartupMessage --> |                  Postgres
//  |                       |                       |                     |
//  |                       |                       | ----- connect ----> |
//  |                       |                       |                     |
//  | <-------------- ReadyForQuery --------------- |                     |
//  |                       |                       |                     |
//  | ------------------------------ Query -----------------------------> |
//  | <---------------------------- DataRow ----------------------------- |
//  | <------------------------- ReadyForQuery -------------------------- |
//  |                       |                       |                     |
//  | ----------------------------- Terminate --------------------------> |
//  |                       |                       |                     |
package postgres
