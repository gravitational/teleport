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

package clusters

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
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

// GetLeafClusters returns leaf clusters
func (c *Cluster) GetLeafClusters(ctx context.Context, rootProxyClient *client.ProxyClient) ([]LeafCluster, error) {
	var (
		remoteClusters []types.RemoteCluster
		err            error
	)
	err = AddMetadataToRetryableError(ctx, func() error {
		remoteClusters, err = rootProxyClient.GetLeafClusters(ctx)
		return trace.Wrap(err)
	})
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
