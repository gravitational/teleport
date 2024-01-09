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
// # Spanner Protocol
//
// Spanner has a REST API and a RPC API.
// The engine only proxies the RPC API.
//
// In Spanner, clients must first establish a [Spanner session], then execute
// RPCs using the created session's ID.
// The session ID is generated when the session is created and returned to the
// client in the response.
//
// # Session creation
//
// A session corresponds to a single Spanner database within a Spanner instance.
// A session request will have an identifier in it like
// "projects/<project>/instances/<instance>/databases/<database>".
//
// When a client creates a new session, the engine will ensure that the
// requested session <project> and <instance> match those in the Teleport db
// object's GCP metadata for project and instance.
// The engine will map --db-user=<gcp iam role name> to a GCP service account,
// and then impersonate that account to get credentials.
//
// Authz for <database> and <gcp iam role> is enforced at session creation time.
// After a session is created, further requests can be made using its secret ID.
//
// # Procedure Calls
//
// Every procedure call will include an ID like "projects/<project>/instances/<instance>/databases/<database>"
// as well. Teleport will check that the ID prefix matches the Teleport db object's
// GCP metadata and then check that the user is authorized to access <database>.
//
// [Spanner RPCs]: https://cloud.google.com/spanner/docs/reference/rpc
// [session]: https://cloud.google.com/spanner/docs/sessions
package spanner
