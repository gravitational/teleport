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
