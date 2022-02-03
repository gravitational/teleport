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
	"strings"

	"github.com/gravitational/trace"
)

// DefaultPort is the Redis default port.
const DefaultPort = "6379"

const (
	URIScheme    = "redis"
	URISchemeSSL = "rediss"
)

// ConnectionOptions defines Redis connection options.
type ConnectionOptions struct {
	cluster bool
	address string
	port    string
}

// ParseRedisURI parses a Redis connection string and returns the parsed
// connection options like address and connection mode.
// ex: rediss://redis.example.com:6379?cluster=true
func ParseRedisURI(uri string) (*ConnectionOptions, error) {
	if uri == "" {
		return nil, trace.BadParameter("Redis uri is empty")
	}

	u, err := url.Parse(uri)
	if err != nil {
		return nil, trace.BadParameter("failed to parse Redis URI: %v", err)
	}

	switch u.Scheme {
	case URIScheme, URISchemeSSL:
	default:
		return nil, trace.BadParameter("Invalid Redis URI scheme: %q. Expected %q or %q.",
			u.Scheme, URIScheme, URISchemeSSL)
	}

	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, trace.BadParameter("failed to parse Redis host: %v", err)
	}

	if port == "" {
		port = DefaultPort
	}

	values := u.Query()
	// Check if cluster mode should be enabled.
	cluster := values.Has("cluster") && strings.EqualFold(values.Get("cluster"), "true")

	return &ConnectionOptions{
		cluster: cluster,
		address: host,
		port:    port,
	}, nil
}
