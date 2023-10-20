// Copyright 2022 Gravitational, Inc
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

package gcp

import (
	"strings"
)

const (
	// gcpEndpointSuffix is hostname suffix used to determine if hostname is a GCP endpoint
	gcpEndpointSuffix = ".googleapis.com"
)

// IsGCPEndpoint returns true if hostname is a GCP endpoint
func IsGCPEndpoint(hostname string) bool {
	return strings.HasSuffix(hostname, gcpEndpointSuffix)
}
