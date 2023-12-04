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

// Package redis implements database access proxy that handles authentication,
// authorization and protocol parsing of connections from Redis clients to
// Redis standalone or Redis clusters.
//
// After accepting a connection from a Redis client and authorizing it, the proxy dials to the database
// service agent over a reverse tunnel which dials to the target Redis instance.
// Unfortunately, Redis 6 (the latest at the moment of writing) only supports password authentication.
// As Teleport doesn't support password authentication we only authenticate Redis user and leave password
// authentication to the client.
//
// In case of authorization failure the command is not passed to the server,
// instead an "access denied" error is sent back to the Redis client in the
// standard RESP message error format.
//
// Redis Cluster
// Teleport supports Redis standalone and cluster instances. In the cluster mode MOV and ASK
// commands are handled internally by go-redis driver, and they are never passed back to
// a connected client.
//
// Config file
// In order to pass additional arguments to configure Redis connection Teleport requires
// using connection URI instead of host + port combination.
// Example:
//
//   - name: "redis-cluster"
//     protocol: "redis"
//     uri: "rediss://redis.example.com:6379?mode=cluster"
package redis
