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
	"iter"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
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
				nodeNameIndex: services.GetCursorForNode,
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

// GetNode finds and returns a node by name and namespace.
func (c *Cache) GetNode(ctx context.Context, namespace, name string) (types.Server, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetNode")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.nodes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		node, err := c.Config.Presence.GetSSHServer(ctx, presencev1.GetSSHServerRequest_builder{Name: name}.Build())
		return node, trace.Wrap(err)
	}

	n, err := rg.store.get(nodeNameIndex, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return n.DeepCopy(), nil
}

// GetSSHServer returns the specified scoped or unscoped node.
func (c *Cache) GetSSHServer(ctx context.Context, req *presencev1.GetSSHServerRequest) (types.Server, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSSHServer")
	defer span.End()

	getter := genericGetter[types.Server, nodeIndex]{
		cache:      c,
		collection: c.collections.nodes,
		index:      nodeNameIndex,
		upstreamGet: func(ctx context.Context, _ string) (types.Server, error) {
			return c.Config.Presence.GetSSHServer(ctx, req)
		},
	}

	out, err := getter.get(ctx, scopes.MakeResourceCursor(req.GetScope(), req.GetName()))
	if out == nil {
		return nil, trace.Wrap(err)
	}
	return out.DeepCopy(), trace.Wrap(err)
}

type getNodesCacheKey struct {
	namespace string
}

// GetNodes is a part of auth.Cache implementation
func (c *Cache) GetNodes(ctx context.Context, namespace string) ([]types.Server, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetNodes")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.nodes)
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

// ListSSHServers returns a page of registered nodes.
func (c *Cache) ListSSHServers(ctx context.Context, req *presencev1.ListSSHServersRequest) ([]types.Server, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListNodes")
	defer span.End()

	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, "", trace.Wrap(err)
	}
	lister := genericLister[types.Server, nodeIndex]{
		cache:      c,
		collection: c.collections.nodes,
		index:      nodeNameIndex,
		upstreamList: func(ctx context.Context, _ int, _ string) ([]types.Server, string, error) {
			return c.Config.Presence.ListSSHServers(ctx, req)
		},
		nextToken: services.GetCursorForNode,
		filter: func(server types.Server) bool {
			return scopes.MatchScope(scopeFilter, server.GetScope())
		},
	}
	return lister.list(ctx, int(req.GetPageSize()), req.GetPageToken())
}

// RangeSSHServers returns a sequence of nodes filtered by the given
// [*presencev1.ListSSHServersRequest].
func (c *Cache) RangeSSHServers(ctx context.Context, req *presencev1.ListSSHServersRequest) iter.Seq2[types.Server, error] {
	ctx, span := c.Tracer.Start(ctx, "cache/RangeSSHServers")
	defer span.End()

	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return stream.Fail[types.Server](trace.Wrap(err))
	}

	lister := genericLister[types.Server, nodeIndex]{
		cache:      c,
		collection: c.collections.nodes,
		index:      nodeNameIndex,
		upstreamList: func(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
			return c.Config.Presence.ListSSHServers(ctx, presencev1.ListSSHServersRequest_builder{
				PageSize:    int32(pageSize),
				PageToken:   pageToken,
				ScopeFilter: scopeFilter,
			}.Build())
		},
		filter: func(server types.Server) bool {
			return scopes.MatchScope(scopeFilter, server.GetScope())
		},
		nextToken: services.GetCursorForNode,
	}

	return lister.Range(ctx, req.GetPageToken(), "")
}

// getNodesWithTTLCache implements TTL-based caching for the GetNodes endpoint.  All nodes that will be returned from the caching layer
// must be cloned to avoid concurrent modification.
func (c *Cache) getNodesWithTTLCache(ctx context.Context) ([]types.Server, error) {
	cachedNodes, err := utils.FnCacheGet(ctx, c.fnCache, getNodesCacheKey{defaults.Namespace}, func(ctx context.Context) ([]types.Server, error) {
		nodes, err := c.Config.Presence.GetNodes(ctx, defaults.Namespace)
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
