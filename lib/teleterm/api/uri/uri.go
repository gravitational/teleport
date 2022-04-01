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

// New creates an instance of ResourceURI
func New(path string) ResourceURI {
	return ResourceURI{
		path: path,
	}
}

// NewClusterURI creates a new cluster URI for given cluster name
func NewClusterURI(clusterName string) ResourceURI {
	return ResourceURI{
		path: fmt.Sprintf("/clusters/%v", clusterName),
	}
}

// ParseClusterURI parses a string and returns cluster URI
func ParseClusterURI(path string) (ResourceURI, error) {
	URI := New(path)
	rootClusterName := URI.GetRootClusterName()
	leafClusterName := URI.GetLeafClusterName()

	if rootClusterName == "" {
		return URI, trace.BadParameter("missing root cluster name")
	}

	clusterURI := NewClusterURI(rootClusterName)
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

// GetRootClusterName returns root cluster name
func (r ResourceURI) GetRootClusterName() string {
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

// String returns string representation of the Resource URI
func (r ResourceURI) String() string {
	return r.path
}
