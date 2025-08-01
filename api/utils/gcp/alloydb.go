// Copyright 2025 Gravitational, Inc.
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

	"github.com/gravitational/trace"
)

// AlloyDBFullInstanceName fully identifies particular AlloyDB instance.
// The "full" is in contrast with the "Instance" field, which is also referenced to as "instance name",
// yet it isn't a globally unique instance identifier.
type AlloyDBFullInstanceName struct {
	// Project is the project ID.
	ProjectID string
	// Location is location, also known as region.
	Location string
	// ClusterID is the cluster ID.
	ClusterID string
	// InstanceID is the instance ID.
	InstanceID string
}

const (
	// AlloyDBScheme is the custom URI scheme used to disambiguate AlloyDB URIs from all others.
	AlloyDBScheme = "alloydb"

	// alloyDBSchemePrefix is AlloyDBScheme with `://`
	alloyDBSchemePrefix = AlloyDBScheme + "://"
)

// IsAlloyDBConnectionURI returns true if the uri can possibly be parsed as AlloyDB connection URI.
//
// It doesn't try to parse it; it merely searches for the presence of the custom `alloydb://` scheme.
func IsAlloyDBConnectionURI(uri string) bool {
	return strings.HasPrefix(uri, alloyDBSchemePrefix)
}

// ParseAlloyDBConnectionURI parses "connection URI" (as it is called by GCP in some places) into the full instance name.
// The URI format requires custom scheme `alloydb://` which we use to disambiguate AlloyDB URIs from all others.
//
// Example URI: alloydb://projects/my-project-123456/locations/europe-west1/clusters/my-cluster/instances/my-instance
func ParseAlloyDBConnectionURI(connectionURI string) (*AlloyDBFullInstanceName, error) {
	if connectionURI == "" {
		return nil, trace.BadParameter("connection URI cannot be empty")
	}

	uriNoPrefix, found := strings.CutPrefix(connectionURI, alloyDBSchemePrefix)
	if !found {
		return nil, trace.BadParameter("invalid connection URI %q: should start with %v", connectionURI, alloyDBSchemePrefix)
	}

	parts := strings.Split(uriNoPrefix, "/")
	if len(parts) != 8 {
		return nil, trace.BadParameter("invalid connection URI %q: wrong number of parts", connectionURI)
	}

	if parts[0] != "projects" || parts[2] != "locations" || parts[4] != "clusters" || parts[6] != "instances" {
		return nil, trace.BadParameter("invalid connection URI %q: incorrect fixed URI elements", connectionURI)
	}

	project, location, cluster, instance := parts[1], parts[3], parts[5], parts[7]

	if project == "" || location == "" || cluster == "" || instance == "" {
		return nil, trace.BadParameter("invalid connection URI %q: missing mandatory variable parts", connectionURI)
	}

	// ? cannot be part of valid instance name; this looks like attempt at query param.
	if strings.Contains(instance, "?") {
		return nil, trace.BadParameter("invalid connection URI %q: query parameters are not accepted", connectionURI)
	}

	return &AlloyDBFullInstanceName{
		ProjectID:  project,
		Location:   location,
		ClusterID:  cluster,
		InstanceID: instance,
	}, nil
}

// AlloyDBEndpointType is AlloyDB endpoint type.
type AlloyDBEndpointType = string

const (
	// AlloyDBEndpointTypePrivate specifies the connection through the private IP address.
	//
	// See: https://cloud.google.com/alloydb/docs/private-ip
	AlloyDBEndpointTypePrivate = AlloyDBEndpointType("private")
	// AlloyDBEndpointTypePSC specifies the connection through the Private Service Connect (PSC) address.
	//
	// See: https://cloud.google.com/alloydb/docs/private-ip
	AlloyDBEndpointTypePSC = AlloyDBEndpointType("psc")
	// AlloyDBEndpointTypePublic specifies the connection through the public IP address.
	//
	// See: https://cloud.google.com/alloydb/docs/connect-public-ip
	AlloyDBEndpointTypePublic = AlloyDBEndpointType("public")
)

// AlloyDBEndpointTypes is the collection of all recognized AlloyDB endpoint types.
var AlloyDBEndpointTypes = []AlloyDBEndpointType{AlloyDBEndpointTypePrivate, AlloyDBEndpointTypePSC, AlloyDBEndpointTypePublic}

// IsAlloyDBKnownEndpointType returns true if the given endpoint type is one of the known ones.
func IsAlloyDBKnownEndpointType(endpointType string) bool {
	switch endpointType {
	case AlloyDBEndpointTypePrivate, AlloyDBEndpointTypePublic, AlloyDBEndpointTypePSC:
		return true
	}
	return false
}
