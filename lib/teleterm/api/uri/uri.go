/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package uri

import (
	"fmt"

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
var pathKubeResourceNamespace = urlpath.New("/clusters/:cluster/kubes/:kubeName/namespaces/:kubeNamespaceName")
var pathLeafKubeResourceNamespace = urlpath.New("/clusters/:cluster/leaves/:leaf/kubes/:kubeName/namespaces/:kubeNamespaceName")
var pathApps = urlpath.New("/clusters/:cluster/apps/:appName")
var pathLeafApps = urlpath.New("/clusters/:cluster/leaves/:leaf/apps/:appName")

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

// IsRoot indicates whether the URI points at a resource that belongs to a root cluster.
func (r ResourceURI) IsRoot() bool {
	// Inspect profile name first to filter our non-cluster URIs.
	return r.GetProfileName() != "" && r.GetLeafClusterName() == ""
}

// IsLeaf indicates whether the URI points at a resource that belongs to a leaf cluster.
func (r ResourceURI) IsLeaf() bool {
	// Inspect profile name first to filter our non-cluster URIs.
	return r.GetProfileName() != "" && r.GetLeafClusterName() != ""
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

// GetKubeResourceNamespace extracts the kube resource namespace from r. Returns an empty string if the path is not a kube resource URI.
func (r ResourceURI) GetKubeResourceNamespace() string {
	result, ok := pathKubeResourceNamespace.Match(r.path)
	if ok {
		return result.Params["kubeNamespaceName"]
	}

	result, ok = pathLeafKubeResourceNamespace.Match(r.path)
	if ok {
		return result.Params["kubeNamespaceName"]
	}

	return ""
}

// GetAppName extracts the app name from r. Returns an empty string if the path is not an app URI.
func (r ResourceURI) GetAppName() string {
	result, ok := pathApps.Match(r.path)
	if ok {
		return result.Params["appName"]
	}

	result, ok = pathLeafApps.Match(r.path)
	if ok {
		return result.Params["appName"]
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

// GetClusterURI strips any resource-specific information other than the cluster
// to which a given resource belongs to.
//
// If called on a root cluster resource URI, it'll return the URI of the root cluster.
// If called on a leaf cluster resource URI, it'll return the URI of the leaf cluster.
// If called on a root cluster URI or a leaf cluster URI, it's a noop.
func (r ResourceURI) GetClusterURI() ResourceURI {
	return r.GetRootClusterURI().AppendLeafCluster(r.GetLeafClusterName())
}

// AppendServer appends server segment to the URI
func (r ResourceURI) AppendServer(id string) ResourceURI {
	r.path = fmt.Sprintf("%v/servers/%v", r.path, id)
	return r
}

// AppendLeafCluster appends leaf cluster segment to the URI if name is not empty.
func (r ResourceURI) AppendLeafCluster(name string) ResourceURI {
	if name == "" {
		return r
	}

	r.path = fmt.Sprintf("%v/leaves/%v", r.path, name)
	return r
}

// AppendKube appends kube segment to the URI
func (r ResourceURI) AppendKube(name string) ResourceURI {
	r.path = fmt.Sprintf("%v/kubes/%v", r.path, name)
	return r
}

// AppendKubeResourceNamespace appends kube resource namespace segment to the URI.
func (r ResourceURI) AppendKubeResourceNamespace(kubeNamespaceName string) ResourceURI {
	r.path = fmt.Sprintf("%v/namespaces/%v", r.path, kubeNamespaceName)
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

// IsApp returns true if URI is an app resource.
func (r ResourceURI) IsApp() bool {
	return r.GetAppName() != ""
}
