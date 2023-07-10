/*
Copyright 2021 Gravitational, Inc.

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

package uri

import (
	"fmt"

	"github.com/gravitational/trace"
	"github.com/ucarion/urlpath"
)

var pathClusters = urlpath.New("/clusters/:cluster/*")
var pathLeafClusters = urlpath.New("/clusters/:cluster/leaves/:leaf/*")
var pathServers = urlpath.New("/clusters/:cluster/servers/:serverUUID")
var pathLeafServers = urlpath.New("/clusters/:cluster/leaves/:leaf/servers/:serverUUID")
var pathDbs = urlpath.New("/clusters/:cluster/dbs/:dbName")
var pathLeafDbs = urlpath.New("/clusters/:cluster/leaves/:leaf/dbs/:dbName")
var pathKubes = urlpath.New("/clusters/:cluster/kubes/:kubeName")
var pathLeafKubes = urlpath.New("/clusters/:cluster/leaves/:leaf/kubes/:kubeName")

// New creates an instance of ResourceURI
func New(path string) ResourceURI {
	return ResourceURI{
		path: path,
	}
}

// NewClusterURI creates a new cluster URI for given cluster name
func NewClusterURI(profileName string) ResourceURI {
	return ResourceURI{
		path: fmt.Sprintf("/clusters/%v", profileName),
	}
}

// ParseClusterURI parses a string and returns a cluster URI.
//
// If given a resource URI, it'll return the URI of the cluster to which the resource belongs to.
// If given a leaf cluster resource URI, it'll return the URI of the leaf cluster.
func ParseClusterURI(path string) (ResourceURI, error) {
	URI := New(path)
	profileName := URI.GetProfileName()
	leafClusterName := URI.GetLeafClusterName()

	if profileName == "" {
		return URI, trace.BadParameter("missing root cluster name")
	}

	clusterURI := NewClusterURI(profileName)
	if leafClusterName != "" {
		clusterURI = clusterURI.AppendLeafCluster(leafClusterName)
	}

	return clusterURI, nil
}

// NewGatewayURI creates a gateway URI for a given ID
func NewGatewayURI(id string) ResourceURI {
	return ResourceURI{
		path: fmt.Sprintf("/gateways/%v", id),
	}
}

// ResourceURI describes resource URI
type ResourceURI struct {
	path string
}

func (r ResourceURI) GetProfileName() string {
	result, ok := pathClusters.Match(r.path + "/")
	if !ok {
		return ""
	}

	return result.Params["cluster"]
}

// GetLeafClusterName returns leaf cluster name
func (r ResourceURI) GetLeafClusterName() string {
	result, ok := pathLeafClusters.Match(r.path + "/")
	if !ok {
		return ""
	}

	return result.Params["leaf"]
}

// GetDbName extracts the database name from r. Returns an empty string if path is not a database URI.
func (r ResourceURI) GetDbName() string {
	result, ok := pathDbs.Match(r.path)
	if ok {
		return result.Params["dbName"]
	}

	result, ok = pathLeafDbs.Match(r.path)
	if ok {
		return result.Params["dbName"]
	}

	return ""
}

// GetKubeName extracts the kube name from r. Returns an empty string if path is not a kube URI.
func (r ResourceURI) GetKubeName() string {
	result, ok := pathKubes.Match(r.path)
	if ok {
		return result.Params["kubeName"]
	}

	result, ok = pathLeafKubes.Match(r.path)
	if ok {
		return result.Params["kubeName"]
	}

	return ""
}

// GetServerUUID extracts the server UUID from r. Returns an empty string if path is not a server URI.
func (r ResourceURI) GetServerUUID() string {
	result, ok := pathServers.Match(r.path)
	if ok {
		return result.Params["serverUUID"]
	}

	result, ok = pathLeafServers.Match(r.path)
	if ok {
		return result.Params["serverUUID"]
	}

	return ""
}

// GetRootClusterURI trims the existing ResourceURI into a URI that points solely at the root cluster.
func (r ResourceURI) GetRootClusterURI() ResourceURI {
	return NewClusterURI(r.GetProfileName())
}

// AppendServer appends server segment to the URI
func (r ResourceURI) AppendServer(id string) ResourceURI {
	r.path = fmt.Sprintf("%v/servers/%v", r.path, id)
	return r
}

// AppendLeafCluster appends leaf cluster segment to the URI
func (r ResourceURI) AppendLeafCluster(name string) ResourceURI {
	r.path = fmt.Sprintf("%v/leaves/%v", r.path, name)
	return r
}

// AppendKube appends kube segment to the URI
func (r ResourceURI) AppendKube(name string) ResourceURI {
	r.path = fmt.Sprintf("%v/kubes/%v", r.path, name)
	return r
}

// AppendDB appends database segment to the URI
func (r ResourceURI) AppendDB(name string) ResourceURI {
	r.path = fmt.Sprintf("%v/dbs/%v", r.path, name)
	return r
}

// AddGateway appends gateway segment to the URI
func (r ResourceURI) AddGateway(id string) ResourceURI {
	r.path = fmt.Sprintf("%v/gateways/%v", r.path, id)
	return r
}

// AppendApp appends app segment to the URI
func (r ResourceURI) AppendApp(name string) ResourceURI {
	r.path = fmt.Sprintf("%v/apps/%v", r.path, name)
	return r
}

// AppendAccessRequest appends access request segment to the URI
func (r ResourceURI) AppendAccessRequest(id string) ResourceURI {
	r.path = fmt.Sprintf("%v/access_requests/%v", r.path, id)
	return r
}

// String returns string representation of the Resource URI
func (r ResourceURI) String() string {
	return r.path
}

// IsDB returns true if URI is a database resource.
func (r ResourceURI) IsDB() bool {
	return r.GetDbName() != ""
}

// IsKube returns true if URI is a kube resource.
func (r ResourceURI) IsKube() bool {
	return r.GetKubeName() != ""
}
