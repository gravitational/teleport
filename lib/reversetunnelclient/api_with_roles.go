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

package reversetunnelclient

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// RemoteClusterGetter is an interface that defines GetRemoteCluster method
type RemoteClusterGetter interface {
	// GetRemoteCluster returns a remote cluster by name
	GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error)
}

// NewClusterGetterWithRoles returns new ClusterGetter wrapper that authorizes
// access to queried clusters.
func NewClusterGetterWithRoles(clusterGetter ClusterGetter, localCluster string, clusterAccessChecker func(types.RemoteCluster) error, remoteClusterGetter RemoteClusterGetter) *ClusterGetterWithRoles {
	return &ClusterGetterWithRoles{
		clusterGetter:        clusterGetter,
		localCluster:         localCluster,
		clusterAccessChecker: clusterAccessChecker,
		remoteClusterGetter:  remoteClusterGetter,
	}
}

// ClusterGetterWithRoles authorizes requests
type ClusterGetterWithRoles struct {
	clusterGetter ClusterGetter

	localCluster string

	// clusterAccessChecker is used to check RBAC permissions.
	clusterAccessChecker func(types.RemoteCluster) error

	remoteClusterGetter RemoteClusterGetter
}

// Clusters returns all connected Teleport clusters.
func (t *ClusterGetterWithRoles) Clusters(ctx context.Context) ([]Cluster, error) {
	clusters, err := t.clusterGetter.Clusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]Cluster, 0, len(clusters))
	for _, cluster := range clusters {
		if t.localCluster == cluster.GetName() {
			out = append(out, cluster)
			continue
		}
		rc, err := t.remoteClusterGetter.GetRemoteCluster(ctx, cluster.GetName())
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			slog.WarnContext(ctx, "Skipping dangling cluster, no remote cluster resource found", "cluster", cluster.GetName())
			continue
		}
		if err := t.clusterAccessChecker(rc); err != nil {
			if !trace.IsAccessDenied(err) {
				return nil, trace.Wrap(err)
			}
			continue
		}
		out = append(out, cluster)
	}
	return out, nil
}

// Cluster returns a cluster matching the provided clusterName
func (t *ClusterGetterWithRoles) Cluster(ctx context.Context, clusterName string) (Cluster, error) {
	cluster, err := t.clusterGetter.Cluster(ctx, clusterName)
	if err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}
	if t.localCluster == cluster.GetName() {
		return cluster, nil
	}
	rc, err := t.remoteClusterGetter.GetRemoteCluster(ctx, clusterName)
	if err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}
	if err := t.clusterAccessChecker(rc); err != nil {
		return nil, utils.OpaqueAccessDenied(err)
	}
	return cluster, nil
}
