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

	"github.com/gravitational/teleport/api/defaults"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

func newNodeCollection(p services.Presence, w types.WatchKind) (*collection[types.Server], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter UsersService")
	}

	return &collection[types.Server]{
		store: newStore(map[string]func(types.Server) string{
			"name": func(u types.Server) string {
				return u.GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Server, error) {
			return p.GetNodes(ctx, defaults.Namespace)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.Server {
			return &types.ServerV2{
				Kind:    types.KindNode,
				Version: types.V2,
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

	rg, err := acquireReadGuard(c, c.collections.nodes.watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		n, err := c.collections.nodes.store.get("name", name)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return n.DeepCopy(), nil
	}

	node, err := c.Config.Presence.GetNode(ctx, namespace, name)
	return node, trace.Wrap(err)
}

type getNodesCacheKey struct {
	namespace string
}

var _ map[getNodesCacheKey]struct{} // compile-time hashability check

// GetNodes is a part of auth.Cache implementation
func (c *Cache) GetNodes(ctx context.Context, namespace string) ([]types.Server, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetNodes")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.nodes.watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		var out []types.Server
		for n := range c.collections.nodes.store.resources("name", "", "") {
			out = append(out, n.DeepCopy())
		}

		return out, nil
	}

	nodes, err := c.getNodesWithTTLCache(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return nodes, nil
}

type listNodesRequest struct {
	Limit               int32
	StartKey            string
	Labels              map[string]string
	PredicateExpression string
	SearchKeywords      []string
	SortBy              types.SortBy
}

func (c *Cache) listNodes(ctx context.Context, req listNodesRequest) ([]types.Server, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/listNodes")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.nodes.watch)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		filter := services.MatchResourceFilter{
			ResourceKind:   types.KindNode,
			Labels:         req.Labels,
			SearchKeywords: req.SearchKeywords,
		}

		if req.PredicateExpression != "" {
			expression, err := services.NewResourceExpression(req.PredicateExpression)
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			filter.PredicateExpression = expression
		}

		// Adjust page size, so it can't be too large.
		pageSize := int(req.Limit)
		if pageSize <= 0 || pageSize > apidefaults.DefaultChunkSize {
			pageSize = apidefaults.DefaultChunkSize
		}

		var out []types.Server
		for n := range c.collections.nodes.store.resources("name", req.StartKey, "") {
			switch match, err := services.MatchResourceByFilters(n, filter, nil /* ignore dup matches */); {
			case err != nil:
				return nil, "", trace.Wrap(err)
			case match:
				if len(out) == pageSize {
					return out, n.GetName(), nil
				}

				out = append(out, n.DeepCopy())
			}
		}

		return out, "", nil
	}

	cachedNodes, err := c.getNodesWithTTLCache(ctx)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	servers := types.Servers(cachedNodes)
	// Since TTLCaching falls back to retrieving all resources upfront, we also support
	// sorting.
	if err := servers.SortByCustom(req.SortBy); err != nil {
		return nil, "", trace.Wrap(err)
	}

	params := local.FakePaginateParams{
		ResourceType:   types.KindNode,
		Limit:          req.Limit,
		Labels:         req.Labels,
		SearchKeywords: req.SearchKeywords,
		StartKey:       req.StartKey,
	}

	if req.PredicateExpression != "" {
		expression, err := services.NewResourceExpression(req.PredicateExpression)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		params.PredicateExpression = expression
	}

	resp, err := local.FakePaginate(servers.AsResources(), params)
	if err != nil {
		return nil, "", nil
	}

	out := make([]types.Server, 0, len(resp.Resources))
	for _, r := range resp.Resources {
		out = append(out, r.(types.Server))
	}

	return out, resp.NextKey, nil
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
