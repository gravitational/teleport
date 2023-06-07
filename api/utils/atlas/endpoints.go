// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package atlas

import (
	"net/url"
	"strings"

	"github.com/gravitational/trace"
)

// IsAtlasEndpoint returns true if the input URI is an MongoDB Atlas endpoint.
func IsAtlasEndpoint(endpoint string) bool {
	return strings.Contains(endpoint, EndpointSuffix)
}

// ParseAtlasEndpoint extracts the database name from the Atlas endpoint.
func ParseAtlasEndpoint(endpoint string) (string, error) {
	// Add a temporary schema to make a valid URL for url.Parse if schema is
	// not found.
	if !strings.Contains(endpoint, "://") {
		endpoint = "schema://" + endpoint
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", trace.Wrap(err)
	}

	parts := strings.Split(parsed.Hostname(), ".")
	if !IsAtlasEndpoint(endpoint) || len(parts) < 3 {
		return "", trace.BadParameter("failed to parse %q as Mongo Atlas endpoint", endpoint)
	}

	return parts[0], nil
}

const (
	// EndpointSuffix is the databases endpoint suffix.
	EndpointSuffix = ".mongodb.net"
)
