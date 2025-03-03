/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

// Package spanner implements GCP Spanner protocol support for database access.
//
// It has the following components:
//
//   - Engine. Runs inside Teleport database service, accepts connections coming
//     from proxy over reverse tunnel and proxies them to GCP Spanner databases.
//
// # Connection flow
//
// New connections arrive as a plain TLS TCP connection, which is handed
// off to the engine's internal grpc server to negotiate grpc inside the TLS
// tunnel.
// The engine avoids returning any error prior to setting up its internal grpc
// server and handing the connection over to it.
// This includes the initial access check, which is instead done on the first
// RPC received.
// In doing so, errors from access checks or otherwise can be sent to the
// client, which helps to avoid an ungraceful connection close and improves
// the user experience when something goes wrong - they can see a descriptive
// message instead of an unhelpful "deadline exceeded" and their client won't
// hopelessly retry on access denied errors or hang until a timeout.
//
// If any RPC is denied by Teleport RBAC, the engine's internal grpc server will
// shutdown.
// This is technically unnecessary, but improves the user experience because
// their client will see the connection closing and wont continue to send RPCs
// over that connection.
// From the client perspective, access denied errors are coming from Spanner
// itself. This would normally just mean a particular RPC is not allowed by
// GCloud's RBAC, but Teleport RBAC is not so fine-grained. As such, it's better
// to just shutdown than let the client software think that there's any hope
// of making any further RPCs.
//
// # Spanner Protocols
//
// Spanner has a REST API and a RPC API.
// The engine only proxies the [Spanner RPC] API.
//
// In Spanner, clients must first request a [Spanner Session], then execute
// RPCs using that session's ID.
// The complete session ID is generated when the session is created and returned
// to the client in the response.
//
// # RPCs
//
// Every RPC will include an ID.
// That ID is usually a session ID, but the session suffix is not there for
// RPCs that request a new session.
// The ID looks like this:
// "projects/<project>/instances/<instance>/databases/<database>[/sessions/<session>]".
// Teleport will check that the ID's project and instance match the Teleport
// db object's GCP metadata and then check that the user is authorized to
// access <database>.
// Whether or not <session> is valid is up to GCloud to check.
// A <session> ID is not a secret, by the way.
//
// # Auditing
//
// When a client connects, the db.session.start event is emitted after the
// first RPC is processed. By virtue of Spanner's session creation flow, this
// will always be either the CreateSessionRequest or BatchCreateSessionsRequest
// RPC for any properly written client. All RPCs are blocked until
// db.session.start is emitted, by design.
// If access is denied for the first RPC, then all RPCs will return the same
// access denied error and there will be no db.session.end nor RPC audit events.
// Otherwise, each RPC including the first RPC will emit a
// db.session.spanner.rpc event.
// That event will also indicate whether Teleport attempted to relay the RPC
// to GCloud Spanner - it will not attempt to do so if Teleport RBAC denies
// access or if there's an issue with marshaling the RPC into an audit event.
//
// [Spanner RPC]: https://cloud.google.com/spanner/docs/reference/rpc
// [Spanner Session]: https://cloud.google.com/spanner/docs/sessions
package spanner
