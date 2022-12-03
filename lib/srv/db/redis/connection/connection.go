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

package connection

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
)

// DefaultPort is the Redis default port.
const DefaultPort = "6379"

const (
	// URIScheme is a Redis scheme: https://www.iana.org/assignments/uri-schemes/prov/redis
	// Teleport always uses Redis connection over TLS.
	URIScheme = "redis"
	// URISchemeTLS is a Redis scheme that uses TLS for database connection: https://www.iana.org/assignments/uri-schemes/prov/rediss
	URISchemeTLS = "rediss"
)

// Mode defines the mode in which Redis is configured. Currently, supported are single and cluster.
type Mode string

const (
	// Standalone mode should be used when connecting to a single Redis instance.
	Standalone Mode = "standalone"
	// Cluster mode should be used when connecting to a Redis Cluster.
	Cluster Mode = "cluster"
)

// Options defines Redis connection options.
type Options struct {
	// Mode defines Redis connection mode like cluster or single instance.
	Mode Mode
	// address of Redis instance.
	Address string
	// port on which Redis expects new connections.
	Port string
}

// ParseRedisAddress parses a Redis connection string and returns the parsed
// connection options like address and connection mode. If port is skipped
// default Redis 6379 is used.
// Correct inputs:
//
//	rediss://redis.example.com:6379?mode=cluster
//	redis://redis.example.com:6379
//	redis.example.com:6379
//
// Incorrect input:
//
//	redis.example.com:6379?mode=cluster
func ParseRedisAddress(addr string) (*Options, error) {
	// Default to the single mode.
	return ParseRedisAddressWithDefaultMode(addr, Standalone)
}

// ParseRedisAddressWithDefaultMode parses a Redis connection string and uses
// the provided default mode if mode is not specified in the address.
func ParseRedisAddressWithDefaultMode(addr string, defaultMode Mode) (*Options, error) {
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
		case URIScheme, URISchemeTLS:
		default:
			return nil, trace.BadParameter("failed to parse Redis address %q, invalid Redis URI scheme: %q. "+
				"Expected %q or %q.", addr, u.Scheme, URIScheme, URISchemeTLS)
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

		// Check if the port can be parsed as a number. net.SplitHostPort() doesn't guarantee that.
		_, err = strconv.Atoi(port)
		if err != nil {
			return nil, trace.BadParameter("failed to parse Redis URL %q, please provide instance address in "+
				"form address:port or rediss://address:port, error: %v", addr, err)
		}

	} else {
		port = DefaultPort
		host = instanceAddr
	}

	values := redisURL.Query()
	// Get additional connections options

	mode := defaultMode
	if values.Has("mode") {
		connMode := strings.ToLower(values.Get("mode"))
		switch Mode(connMode) {
		case Standalone:
			mode = Standalone
		case Cluster:
			mode = Cluster
		default:
			return nil, trace.BadParameter("incorrect connection mode %q, supported are: %s and %s",
				connMode, Standalone, Cluster)
		}
	}

	return &Options{
		Mode:    mode,
		Address: host,
		Port:    port,
	}, nil
}
