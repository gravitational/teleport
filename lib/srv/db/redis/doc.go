/*
Copyright 2022 Gravitational, Inc.

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
//  - name: "redis-cluster"
//    protocol: "redis"
//    uri: "rediss://redis.example.com:6379?mode=cluster"
package redis
