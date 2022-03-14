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

package db

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
)

// RedisDefaultPort is the Redis default Port.
const RedisDefaultPort = "6379"

const (
	// RedisURIScheme is a Redis scheme: https://www.iana.org/assignments/uri-schemes/prov/redis
	// Teleport always uses Redis connection over TLS.
	RedisURIScheme = "redis"
	// RedisURISchemeTLS is a Redis scheme that uses TLS for database connection: https://www.iana.org/assignments/uri-schemes/prov/rediss
	RedisURISchemeTLS = "rediss"
)

// RedisConnectionMode defines the Mode in which Redis is configured. Currently, supported are single and cluster.
type RedisConnectionMode string

const (
	// Standalone Mode should be used when connecting to a single Redis instance.
	Standalone RedisConnectionMode = "standalone"
	// Cluster Mode should be used when connecting to a Redis Cluster.
	Cluster RedisConnectionMode = "cluster"
)

// RedisConnectionOptions defines Redis connection options.
type RedisConnectionOptions struct {
	// Mode defines Redis connection Mode like cluster or single instance.
	Mode RedisConnectionMode
	// Host of Redis instance.
	Host string
	// Port on which Redis expects new connections.
	Port string
}

// ParseRedisAddress parses a Redis connection string and returns the parsed
// connection options like host, port and connection mode. If port is skipped
// default Redis 6379 port is used.
// Correct inputs:
// 	rediss://redis.example.com:6379?mode=cluster
// 	redis://redis.example.com:6379
// 	redis.example.com:6379
//
// Incorrect input:
//	redis.example.com:6379?mode=cluster
func ParseRedisAddress(addr string) (*RedisConnectionOptions, error) {
	if addr == "" {
		return nil, trace.BadParameter("Redis address is empty")
	}

	var redisURL url.URL
	var instanceAddr string

	if strings.Contains(addr, "://") {
		// Assume URI version
		u, err := url.Parse(addr)
		if err != nil {
			return nil, trace.BadParameter("failed to parse Redis URI: %q, error: %v", addr, err)
		}

		switch u.Scheme {
		case RedisURIScheme, RedisURISchemeTLS:
		default:
			return nil, trace.BadParameter("failed to parse Redis Address %q, invalid Redis URI scheme: %q. "+
				"Expected %q or %q.", addr, u.Scheme, RedisURIScheme, RedisURISchemeTLS)
		}

		redisURL = *u
		instanceAddr = u.Host
	} else {
		// Assume host:port pair
		instanceAddr = addr
	}

	var (
		host string
		port string
	)

	if strings.Contains(instanceAddr, ":") {
		var err error
		host, port, err = net.SplitHostPort(instanceAddr)
		if err != nil {
			return nil, trace.BadParameter("failed to parse Redis host: %q, error: %v", addr, err)
		}

		// Check if the Port can be parsed as a number. net.SplitHostPort() doesn't guarantee that.
		_, err = strconv.Atoi(port)
		if err != nil {
			return nil, trace.BadParameter("failed to parse Redis URL %q, please provide instance address in "+
				"form host:port or rediss://host:port, error: %v", addr, err)
		}

	} else {
		port = RedisDefaultPort
		host = instanceAddr
	}

	values := redisURL.Query()
	// Get additional connections options

	// Default to the single Mode.
	mode := Standalone
	if values.Has("mode") {
		connMode := strings.ToLower(values.Get("mode"))
		switch RedisConnectionMode(connMode) {
		case Standalone:
			mode = Standalone
		case Cluster:
			mode = Cluster
		default:
			return nil, trace.BadParameter("incorrect connection Mode %q, supported are: %s and %s",
				connMode, Standalone, Cluster)
		}
	}

	return &RedisConnectionOptions{
		Mode: mode,
		Host: host,
		Port: port,
	}, nil
}
