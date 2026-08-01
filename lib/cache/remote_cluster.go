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
	oteltrace "go.opentelemetry.io/otel/trace"

	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/cache/internal"
	"github.com/gravitational/teleport/lib/itertools/stream"
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
			types.KindTunnelConnection,
			types.TunnelConnection.Clone,
			map[tunnelConnectionIndex]func(types.TunnelConnection) string{
				tunnelConnectionNameIndex: func(tc types.TunnelConnection) string {
					return tc.GetClusterName() + "/" + tc.GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.TunnelConnection, error) {
			out, err := upstream.GetAllTunnelConnections(ctx)
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

// tunnelConnectionCollection provides read access to cached tunnel
// connections. Its exported methods are promoted onto every topology cache
// that embeds it; the reads are implemented exactly once here. It is a
// stateless value assembled inline by each of its consumers so that no
// shared scaffolding couples their lifetimes.
type tunnelConnectionCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.Trust
	col      *collection[types.TunnelConnection, tunnelConnectionIndex]
}

// GetTunnelConnections is a part of auth.Cache implementation
func (c tunnelConnectionCollection) GetTunnelConnections(ctx context.Context, clusterName string) ([]types.TunnelConnection, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetTunnelConnections")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		tunnels, err := c.upstream.GetTunnelConnections(ctx, clusterName)
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

// GetTunnelConnections is a part of auth.Cache implementation
func (c *Cache) GetTunnelConnections(ctx context.Context, clusterName string) ([]types.TunnelConnection, error) {
	return tunnelConnectionCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Trust,
		col:      c.collections.tunnelConnections,
	}.GetTunnelConnections(ctx, clusterName)
}

// GetAllTunnelConnections is a part of auth.Cache implementation
func (c tunnelConnectionCollection) GetAllTunnelConnections(ctx context.Context) (conns []types.TunnelConnection, err error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetAllTunnelConnections")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		tunnels, err := c.upstream.GetAllTunnelConnections(ctx)
		return tunnels, trace.Wrap(err)
	}

	tunnels := make([]types.TunnelConnection, 0, rg.store.len())
	for t := range rg.store.resources(tunnelConnectionNameIndex, "", "") {
		tunnels = append(tunnels, t.Clone())
	}

	return tunnels, nil
}

// GetAllTunnelConnections is a part of auth.Cache implementation
func (c *Cache) GetAllTunnelConnections(ctx context.Context) (conns []types.TunnelConnection, err error) {
	return tunnelConnectionCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Trust,
		col:      c.collections.tunnelConnections,
	}.GetAllTunnelConnections(ctx)
}

// ListTunnelConnections returns a page of tunnel connections matching the
// given filter.
func (c tunnelConnectionCollection) ListTunnelConnections(ctx context.Context, pageSize int, pageToken string, filter *trustpb.ListTunnelConnectionsFilter) ([]types.TunnelConnection, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListTunnelConnections")
	defer span.End()

	lister := genericLister[types.TunnelConnection, tunnelConnectionIndex]{
		engine:     c.engine,
		collection: c.col,
		index:      tunnelConnectionNameIndex,
		upstreamList: func(ctx context.Context, pageSize int, pageToken string) ([]types.TunnelConnection, string, error) {
			return c.upstream.ListTunnelConnections(ctx, pageSize, pageToken, filter)
		},
		nextToken: func(tc types.TunnelConnection) string {
			return tc.GetClusterName() + "/" + tc.GetName()
		},
	}

	if clusterName := filter.GetClusterName(); clusterName != "" {
		startToken := clusterName + "/"
		if pageToken != "" {
			startToken = pageToken
		}
		endToken := sortcache.NextKey(clusterName + "/")
		out, next, err := lister.listRange(ctx, pageSize, startToken, endToken)
		return out, next, trace.Wrap(err)
	}

	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// ListTunnelConnections returns a page of tunnel connections matching the
// given filter.
func (c *Cache) ListTunnelConnections(ctx context.Context, pageSize int, pageToken string, filter *trustpb.ListTunnelConnectionsFilter) ([]types.TunnelConnection, string, error) {
	return tunnelConnectionCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Trust,
		col:      c.collections.tunnelConnections,
	}.ListTunnelConnections(ctx, pageSize, pageToken, filter)
}

type remoteClusterIndex string

const remoteClusterNameIndex remoteClusterIndex = "name"

func newRemoteClusterCollection(upstream services.Trust, w types.WatchKind) (*collection[types.RemoteCluster, remoteClusterIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Trust")
	}

	return &collection[types.RemoteCluster, remoteClusterIndex]{
		store: newStore(
			types.KindRemoteCluster,
			types.RemoteCluster.Clone,
			map[remoteClusterIndex]func(types.RemoteCluster) string{
				remoteClusterNameIndex: types.RemoteCluster.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.RemoteCluster, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListRemoteClusters))
			return out, trace.Wrap(err)
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

// remoteClusterCollection provides read access to cached remote clusters.
// Its exported methods are promoted onto every topology cache that embeds
// it; the reads are implemented exactly once here. It is a stateless value
// assembled inline by each of its consumers so that no shared scaffolding
// couples their lifetimes.
type remoteClusterCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	fnCache  *utils.FnCache
	upstream services.Trust
	col      *collection[types.RemoteCluster, remoteClusterIndex]
}

// GetRemoteClusters returns a list of remote clusters
func (c remoteClusterCollection) GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetRemoteClusters")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
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
			clusters, next, err := c.upstream.ListRemoteClusters(ctx, 0, startKey)
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

// GetRemoteClusters returns a list of remote clusters
func (c *Cache) GetRemoteClusters(ctx context.Context) ([]types.RemoteCluster, error) {
	return remoteClusterCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		fnCache:  c.fnCache,
		upstream: c.Config.Trust,
		col:      c.collections.remoteClusters,
	}.GetRemoteClusters(ctx)
}

// GetRemoteCluster returns a remote cluster by name
func (c remoteClusterCollection) GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetRemoteCluster")
	defer span.End()

	var upstreamRead bool
	getter := genericGetter[types.RemoteCluster, remoteClusterIndex]{
		engine:     c.engine,
		collection: c.col,
		index:      remoteClusterNameIndex,
		upstreamGet: func(ctx context.Context, clusterName string) (types.RemoteCluster, error) {
			upstreamRead = true
			cachedRemote, err := utils.FnCacheGet(ctx, c.fnCache, remoteClustersCacheKey{clusterName}, func(ctx context.Context) (types.RemoteCluster, error) {
				remote, err := c.upstream.GetRemoteCluster(ctx, clusterName)
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
		if rc, err := c.upstream.GetRemoteCluster(ctx, clusterName); err == nil {
			return rc, nil
		}
	}
	return out, trace.Wrap(err)
}

// GetRemoteCluster returns a remote cluster by name
func (c *Cache) GetRemoteCluster(ctx context.Context, clusterName string) (types.RemoteCluster, error) {
	return remoteClusterCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		fnCache:  c.fnCache,
		upstream: c.Config.Trust,
		col:      c.collections.remoteClusters,
	}.GetRemoteCluster(ctx, clusterName)
}

// ListRemoteClusters returns a page of remote clusters.
func (c remoteClusterCollection) ListRemoteClusters(ctx context.Context, pageSize int, nextToken string) ([]types.RemoteCluster, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListRemoteClusters")
	defer span.End()

	lister := genericLister[types.RemoteCluster, remoteClusterIndex]{
		engine:       c.engine,
		collection:   c.col,
		index:        remoteClusterNameIndex,
		upstreamList: c.upstream.ListRemoteClusters,
		nextToken:    types.RemoteCluster.GetName,
	}
	out, next, err := lister.list(ctx, pageSize, nextToken)
	return out, next, trace.Wrap(err)
}

// ListRemoteClusters returns a page of remote clusters.
func (c *Cache) ListRemoteClusters(ctx context.Context, pageSize int, nextToken string) ([]types.RemoteCluster, string, error) {
	return remoteClusterCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		fnCache:  c.fnCache,
		upstream: c.Config.Trust,
		col:      c.collections.remoteClusters,
	}.ListRemoteClusters(ctx, pageSize, nextToken)
}
