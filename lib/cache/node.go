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

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cache/internal"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type nodeIndex string

const nodeNameIndex nodeIndex = "name"

func newNodeCollection(p services.Presence, w types.WatchKind) (*collection[types.Server, nodeIndex], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[types.Server, nodeIndex]{
		store: newStore(
			types.KindNode,
			types.Server.DeepCopy,
			map[nodeIndex]func(types.Server) string{
				nodeNameIndex: types.Server.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Server, error) {
			return p.GetNodes(ctx, defaults.Namespace)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.Server {
			return &types.ServerV2{
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

// nodeCollection provides read access to cached nodes. Its exported methods
// are promoted onto every topology cache that embeds it; the reads are
// implemented exactly once here. It is a stateless value assembled inline by
// each of its consumers so that no shared scaffolding couples their
// lifetimes.
type nodeCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	fnCache  *utils.FnCache
	upstream services.Presence
	col      *collection[types.Server, nodeIndex]
}

// GetNode finds and returns a node by name and namespace.
func (c nodeCollection) GetNode(ctx context.Context, namespace, name string) (types.Server, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetNode")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		node, err := c.upstream.GetNode(ctx, namespace, name)
		return node, trace.Wrap(err)
	}

	n, err := rg.store.get(nodeNameIndex, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return n.DeepCopy(), nil
}

// GetNode finds and returns a node by name and namespace.
func (c *Cache) GetNode(ctx context.Context, namespace, name string) (types.Server, error) {
	return nodeCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		fnCache:  c.fnCache,
		upstream: c.Config.Presence,
		col:      c.collections.nodes,
	}.GetNode(ctx, namespace, name)
}

type getNodesCacheKey struct {
	namespace string
}

// GetNodes is a part of auth.Cache implementation
func (c nodeCollection) GetNodes(ctx context.Context, namespace string) ([]types.Server, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetNodes")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		nodes, err := c.getNodesWithTTLCache(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return nodes, nil
	}

	out := make([]types.Server, 0, rg.store.len())
	for n := range rg.store.resources(nodeNameIndex, "", "") {
		out = append(out, n.DeepCopy())
	}

	return out, nil

}

// GetNodes is a part of auth.Cache implementation
func (c *Cache) GetNodes(ctx context.Context, namespace string) ([]types.Server, error) {
	return nodeCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		fnCache:  c.fnCache,
		upstream: c.Config.Presence,
		col:      c.collections.nodes,
	}.GetNodes(ctx, namespace)
}

// getNodesWithTTLCache implements TTL-based caching for the GetNodes endpoint.  All nodes that will be returned from the caching layer
// must be cloned to avoid concurrent modification.
func (c nodeCollection) getNodesWithTTLCache(ctx context.Context) ([]types.Server, error) {
	cachedNodes, err := utils.FnCacheGet(ctx, c.fnCache, getNodesCacheKey{defaults.Namespace}, func(ctx context.Context) ([]types.Server, error) {
		nodes, err := c.upstream.GetNodes(ctx, defaults.Namespace)
		return nodes, err
	})

	// Nodes returned from the TTL caching layer
	// must be cloned to avoid concurrent modification.
	clonedNodes := make([]types.Server, 0, len(cachedNodes))
	for _, node := range cachedNodes {
		clonedNodes = append(clonedNodes, node.DeepCopy())
	}
	return clonedNodes, trace.Wrap(err)
}

// getNodesWithTTLCache implements TTL-based caching for the GetNodes endpoint.  All nodes that will be returned from the caching layer
// must be cloned to avoid concurrent modification.
func (c *Cache) getNodesWithTTLCache(ctx context.Context) ([]types.Server, error) {
	return nodeCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		fnCache:  c.fnCache,
		upstream: c.Config.Presence,
		col:      c.collections.nodes,
	}.getNodesWithTTLCache(ctx)
}
