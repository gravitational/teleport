// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/sortcache"
)

type tunnelConnectionIndex string

const tunnelConnectionNameIndex tunnelConnectionIndex = "name"

func newTunnelConnectionCollection(upstream services.Trust, w types.WatchKind) (*collection[types.TunnelConnection, tunnelConnectionIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Trust")
	}

	return &collection[types.TunnelConnection, tunnelConnectionIndex]{
		store: newStore(
			types.TunnelConnection.Clone,
			map[tunnelConnectionIndex]func(types.TunnelConnection) string{
				tunnelConnectionNameIndex: func(tc types.TunnelConnection) string {
					return tc.GetClusterName() + "/" + tc.GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.TunnelConnection, error) {
			out, err := upstream.GetAllTunnelConnections()
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.TunnelConnection {
			return &types.TunnelConnectionV2{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
				Spec: types.TunnelConnectionSpecV2{
					ClusterName: hdr.SubKind,
				},
			}
		},
		watch: w,
	}, nil
}

// GetTunnelConnections is a part of auth.Cache implementation
func (c *Cache) GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]types.TunnelConnection, error) {
	_, span := c.Tracer.Start(context.TODO(), "cache/GetTunnelConnections")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.tunnelConnections)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		tunnels, err := c.Config.Trust.GetTunnelConnections(clusterName, opts...)
		return tunnels, trace.Wrap(err)
	}

	startKey := clusterName + "/"
	endKey := sortcache.NextKey(startKey)
	var tunnels []types.TunnelConnection
	for t := range rg.store.resources(tunnelConnectionNameIndex, startKey, endKey) {
		tunnels = append(tunnels, t.Clone())
	}

	return tunnels, nil
}

// GetAllTunnelConnections is a part of auth.Cache implementation
func (c *Cache) GetAllTunnelConnections(opts ...services.MarshalOption) (conns []types.TunnelConnection, err error) {
	_, span := c.Tracer.Start(context.TODO(), "cache/GetAllTunnelConnections")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.tunnelConnections)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		tunnels, err := c.Config.Trust.GetAllTunnelConnections(opts...)
		return tunnels, trace.Wrap(err)
	}

	tunnels := make([]types.TunnelConnection, 0, rg.store.len())
	for t := range rg.store.resources(tunnelConnectionNameIndex, "", "") {
		tunnels = append(tunnels, t.Clone())
	}

	return tunnels, nil
}

type remoteClusterIndex string

const remoteClusterNameIndex remoteClusterIndex = "name"

func newRemoteClusterCollection(upstream services.Trust, w types.WatchKind) (*collection[types.RemoteCluster, remoteClusterIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Trust")
	}

	return &collection[types.RemoteCluster, remoteClusterIndex]{
		store: newStore(
			types.RemoteCluster.Clone,
			map[remoteClusterIndex]func(types.RemoteCluster) string{
				remoteClusterNameIndex: types.RemoteCluster.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.RemoteCluster, error) {
			var out []types.RemoteCluster
			var startKey string

			for {
				clusters, next, err := upstream.ListRemoteClusters(ctx, 0, startKey)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				out = append(out, clusters...)
				startKey = next
				if next == "" {
					break
				}
			}

			return out, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.RemoteCluster {
			return &types.RemoteClusterV3{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

type remoteClustersCacheKey struct {
	name string
}

// GetRemoteClusters returns a list of remote clusters
func (c *Cache) GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetRemoteClusters")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.remoteClusters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		remotes := make([]types.RemoteCluster, 0, rg.store.len())
		for rc := range rg.store.resources(remoteClusterNameIndex, "", "") {
			remotes = append(remotes, rc.Clone())
		}

		return remotes, nil
	}

	cachedRemotes, err := utils.FnCacheGet(ctx, c.fnCache, remoteClustersCacheKey{}, func(ctx context.Context) ([]types.RemoteCluster, error) {
		var out []types.RemoteCluster
		var startKey string

		for {
			clusters, next, err := c.Config.Trust.ListRemoteClusters(ctx, 0, startKey)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			out = append(out, clusters...)
			startKey = next
			if next == "" {
				break
			}
		}

		return out, nil
	})
	if err != nil || cachedRemotes == nil {
		return nil, trace.Wrap(err)
	}

	remotes := make([]types.RemoteCluster, 0, len(cachedRemotes))
	for _, remote := range cachedRemotes {
		remotes = append(remotes, remote.Clone())
	}
	return remotes, nil
}

// GetRemoteCluster returns a remote cluster by name
func (c *Cache) GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetRemoteCluster")
	defer span.End()

	var upstreamRead bool
	getter := genericGetter[types.RemoteCluster, remoteClusterIndex]{
		cache:      c,
		collection: c.collections.remoteClusters,
		index:      remoteClusterNameIndex,
		upstreamGet: func(ctx context.Context, clusterName string) (types.RemoteCluster, error) {
			upstreamRead = true
			cachedRemote, err := utils.FnCacheGet(ctx, c.fnCache, remoteClustersCacheKey{clusterName}, func(ctx context.Context) (types.RemoteCluster, error) {
				remote, err := c.Config.Trust.GetRemoteCluster(ctx, clusterName)
				return remote, err
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return cachedRemote.Clone(), nil
		},
	}
	out, err := getter.get(ctx, clusterName)
	if trace.IsNotFound(err) && !upstreamRead {
		// fallback is sane because this method is never used
		// in construction of derivative caches.
		if rc, err := c.Config.Trust.GetRemoteCluster(ctx, clusterName); err == nil {
			return rc, nil
		}
	}
	return out, trace.Wrap(err)
}

// ListRemoteClusters returns a page of remote clusters.
func (c *Cache) ListRemoteClusters(ctx context.Context, pageSize int, nextToken string) ([]types.RemoteCluster, string, error) {
	_, span := c.Tracer.Start(ctx, "cache/ListRemoteClusters")
	defer span.End()

	lister := genericLister[types.RemoteCluster, remoteClusterIndex]{
		cache:        c,
		collection:   c.collections.remoteClusters,
		index:        remoteClusterNameIndex,
		upstreamList: c.Config.Trust.ListRemoteClusters,
		nextToken:    types.RemoteCluster.GetName,
	}
	out, next, err := lister.list(ctx, pageSize, nextToken)
	return out, next, trace.Wrap(err)
}
