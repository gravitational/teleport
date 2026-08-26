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

// Package mongodb implements database access proxy that handles authentication,
// authorization and protocol parsing of connections from MongoDB clients to
// MongoDB clusters.
//
// After accepting a connection from a MongoDB client and authorizing it, the
// proxy dials to the target MongoDB cluster, performs x509 authentication and
// starts relaying wire messages between client and server.
//
// Server selection
// ================
// When connecting to a MongoDB replica set, the proxy will establish connection
// to the server determined by the "readPreference" setting from the config's
// connection string.
//
// For example, this configuration will make Teleport to connect to a secondary:
//
//   - name: "mongo-rs"
//     protocol: "mongodb"
//     uri: "mongodb://mongo1:27017,mongo2:27017/?replicaSet=rs0&readPreference=secondary"
//
// Command authorization
// =====================
// Each MongoDB command is executed in a particular database. Client commands
// going through the proxy are inspected and their database is checked against
// user role's "db_names".
//
// In case of authorization failure the command is not passed to the server,
// instead an "access denied" error is sent back to the MongoDB client in the
// standard wire message error format.
package mongodb
