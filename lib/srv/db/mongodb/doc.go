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
