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
	"fmt"
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

// ParentClusterName returns the full name of parent cluster.
func (info AlloyDBFullInstanceName) ParentClusterName() string {
	return fmt.Sprintf(
		"projects/%s/locations/%s/clusters/%s", info.ProjectID, info.Location, info.ClusterID,
	)
}

// InstanceName returns a full name of the instance.
func (info AlloyDBFullInstanceName) InstanceName() string {
	return fmt.Sprintf(
		"projects/%s/locations/%s/clusters/%s/instances/%s", info.ProjectID, info.Location, info.ClusterID, info.InstanceID,
	)
}

const (
	// alloyDBScheme is the custom URI scheme used to disambiguate AlloyDB URIs from all others.
	alloyDBScheme = "alloydb://"
)

// IsAlloyDBConnectionURI returns true if the uri can possibly be parsed as AlloyDB connection URI.
//
// It doesn't try to parse it; it merely searches for the presence of the custom `alloydb://` scheme.
func IsAlloyDBConnectionURI(uri string) bool {
	return strings.HasPrefix(uri, alloyDBScheme)
}

// ParseAlloyDBConnectionURI parses "connection URI" (as it is called by GCP in some places) into the full instance name.
// The URI format requires custom scheme `alloydb://` which we use to disambiguate AlloyDB URIs from all others.
//
// Example URI: alloydb://projects/my-project-123456/locations/europe-west1/clusters/my-cluster/instances/my-instance
func ParseAlloyDBConnectionURI(connectionURI string) (*AlloyDBFullInstanceName, error) {
	if connectionURI == "" {
		return nil, trace.BadParameter("connection URI cannot be empty")
	}

	uriNoPrefix, found := strings.CutPrefix(connectionURI, alloyDBScheme)
	if !found {
		return nil, trace.BadParameter("invalid connection URI %q: should start with %v", connectionURI, alloyDBScheme)
	}

	parts := strings.Split(uriNoPrefix, "/")
	if len(parts) != 8 {
		return nil, trace.BadParameter("invalid connection URI %q: wrong number of parts", connectionURI)
	}

	switch {
	case parts[0] != "projects":
		return nil, trace.BadParameter("invalid connection URI %q: expected 'projects', got %q", connectionURI, parts[0])
	case parts[2] != "locations":
		return nil, trace.BadParameter("invalid connection URI %q: expected 'locations', got %q", connectionURI, parts[2])
	case parts[4] != "clusters":
		return nil, trace.BadParameter("invalid connection URI %q: expected 'clusters', got %q", connectionURI, parts[4])
	case parts[6] != "instances":
		return nil, trace.BadParameter("invalid connection URI %q: expected 'instances', got %q", connectionURI, parts[6])
	}

	project, location, cluster, instance := parts[1], parts[3], parts[5], parts[7]

	switch {
	case project == "":
		return nil, trace.BadParameter("invalid connection URI %q: project cannot be empty", connectionURI)
	case location == "":
		return nil, trace.BadParameter("invalid connection URI %q: location cannot be empty", connectionURI)
	case cluster == "":
		return nil, trace.BadParameter("invalid connection URI %q: cluster cannot be empty", connectionURI)
	case instance == "":
		return nil, trace.BadParameter("invalid connection URI %q: instance cannot be empty", connectionURI)
	}

	// '?' cannot be a part of a valid instance name; this looks like attempt at query param.
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
type AlloyDBEndpointType string

const (
	// AlloyDBEndpointTypePublic is the public endpoint type.
	AlloyDBEndpointTypePublic AlloyDBEndpointType = "public"
	// AlloyDBEndpointTypePrivate is the private endpoint type.
	AlloyDBEndpointTypePrivate AlloyDBEndpointType = "private"
	// AlloyDBEndpointTypePSC is the PSC endpoint type.
	AlloyDBEndpointTypePSC AlloyDBEndpointType = "psc"
)

// AlloyDBEndpointTypes is a list of all known AlloyDB endpoint types.
var AlloyDBEndpointTypes = []AlloyDBEndpointType{
	AlloyDBEndpointTypePublic,
	AlloyDBEndpointTypePrivate,
	AlloyDBEndpointTypePSC,
}

func ValidateAlloyDBEndpointType(endpointType string) error {
	if endpointType == "" {
		return nil
	}
	for _, t := range AlloyDBEndpointTypes {
		if endpointType == string(t) {
			return nil
		}
	}
	return trace.BadParameter("invalid alloy db endpoint type: %v, expected one of %v", endpointType, AlloyDBEndpointTypes)
}
