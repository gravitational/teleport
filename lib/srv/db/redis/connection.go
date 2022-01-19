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

	"github.com/gravitational/trace"
)

type ConnectionOptions struct {
	cluster bool
	address string
	port    string
}

func ParseRedisURI(uri string) (*ConnectionOptions, error) {
	if uri == "" {
		return nil, trace.BadParameter("Redis uri is empty")
	}

	u, err := url.Parse(uri)
	if err != nil {
		return nil, trace.BadParameter("failed to parse Redis URI: %v", err)
	}

	switch u.Scheme {
	case "redis", "rediss":
	default:
		return nil, trace.BadParameter("Redis schema protocol is incorrect, expected redis or rediss, provided %q", u.Scheme)
	}

	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, trace.BadParameter("failed to parse Redis host: %v", err)
	}

	if port == "" {
		port = "6379"
	}

	cluster := false

	values := u.Query()
	if values.Has("cluster") && values.Get("cluster") == "true" { // TODO(jakub): make it more generic
		cluster = true
	}

	return &ConnectionOptions{
		cluster: cluster,
		address: host,
		port:    port,
	}, nil
}
