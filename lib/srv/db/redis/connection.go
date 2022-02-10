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

package redis

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
	URIScheme    = "redis"
	URISchemeSSL = "rediss"
)

// ConnectionMode defines the mode in which Redis is configured. Currently, supported are single and cluster.
type ConnectionMode string

const (
	// Standalone mode should be used when connecting to a single Redis instance.
	Standalone ConnectionMode = "standalone"
	// Cluster mode should be used then connecting to a Redis Cluster.
	Cluster ConnectionMode = "cluster"
)

// ConnectionOptions defines Redis connection options.
type ConnectionOptions struct {
	// mode defines Redis connection mode like cluster or single instance.
	mode ConnectionMode
	// address of Redis instance.
	address string
	// port on which Redis expects new connections.
	port string
}

// ParseRedisURI parses a Redis connection string and returns the parsed
// connection options like address and connection mode. If port is skipped
// default Redis 6379 is used.
// Correct inputs:
// 	rediss://redis.example.com:6379?mode=cluster
// 	redis://redis.example.com:6379
// 	redis.example.com:6379
//
// Incorrect input:
//	redis.example.com:6379?mode=cluster
func ParseRedisURI(uri string) (*ConnectionOptions, error) {
	if uri == "" {
		return nil, trace.BadParameter("Redis uri is empty")
	}

	var redisURL url.URL
	var instanceAddr string

	if strings.Contains(uri, "://") {
		// Assume URI version
		u, err := url.Parse(uri)
		if err != nil {
			return nil, trace.BadParameter("failed to parse Redis URI: %v", err)
		}

		switch u.Scheme {
		case URIScheme, URISchemeSSL:
		default:
			return nil, trace.BadParameter("failed to parse Redis address %q, invalid Redis URI scheme: %q. "+
				"Expected %q or %q.", uri, u.Scheme, URIScheme, URISchemeSSL)
		}

		redisURL = *u
		instanceAddr = u.Host
	} else {
		// Assume host:port pair
		instanceAddr = uri
	}

	var (
		host string
		port string
	)

	if strings.Contains(instanceAddr, ":") {
		var err error
		host, port, err = net.SplitHostPort(instanceAddr)
		if err != nil {
			return nil, trace.BadParameter("failed to parse Redis host: %v", err)
		}

		// Check if the port can be parsed as a number. net.SplitHostPort() doesn't guarantee that.
		_, err = strconv.Atoi(port)
		if err != nil {
			return nil, trace.BadParameter("failed to parse Redis URL %q, please provide instance address in "+
				"form address:port or rediss://address:port. Error: %v", uri, err)
		}

	} else {
		port = DefaultPort
		host = instanceAddr
	}

	values := redisURL.Query()
	// Get additional connections options

	// Default to the single mode.
	mode := Standalone
	if values.Has("mode") {
		connMode := strings.ToLower(values.Get("mode"))
		switch ConnectionMode(connMode) {
		case Standalone:
			mode = Standalone
		case Cluster:
			mode = Cluster
		default:
			return nil, trace.BadParameter("incorrect connection mode %q, supported are: %s and %s",
				connMode, Standalone, Cluster)
		}
	}

	return &ConnectionOptions{
		mode:    mode,
		address: host,
		port:    port,
	}, nil
}
