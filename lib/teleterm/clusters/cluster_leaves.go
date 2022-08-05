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

package clusters

import (
	"context"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"

	"github.com/gravitational/trace"
)

// LeafCluster describes a leaf (trusted) cluster
type LeafCluster struct {
	// URI is the leaf cluster URI
	URI uri.ResourceURI
	// LoggedInUser is the logged in user
	LoggedInUser LoggedInUser
	// Name is the leaf cluster name
	Name string
	// Connected indicates if this leaf cluster is connected
	Connected bool
}

//GetLeafClusters returns leaf clusters
func (c *Cluster) GetLeafClusters(ctx context.Context) ([]LeafCluster, error) {
	proxyClient, err := c.clusterClient.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	remoteClusters, err := proxyClient.GetLeafClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results := []LeafCluster{}
	for _, remoteCluster := range remoteClusters {
		results = append(results, LeafCluster{
			URI:          c.URI.AppendLeafCluster(remoteCluster.GetName()),
			Name:         remoteCluster.GetName(),
			Connected:    remoteCluster.GetConnectionStatus() == teleport.RemoteClusterStatusOnline,
			LoggedInUser: c.GetLoggedInUser(),
		})
	}

	return results, nil
}
