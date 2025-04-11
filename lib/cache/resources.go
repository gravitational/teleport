package cache

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

// ListResources is a part of auth.Cache implementation
func (c *Cache) ListResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListResources")
	defer span.End()

	if c.closed.Load() {
		return nil, trace.Errorf("cache is closed")
	}
	c.rw.RLock()

	kind := types.WatchKind{Kind: req.ResourceType}
	_, kindOK := c.confirmedKinds[resourceKind{kind: kind.Kind, subkind: kind.SubKind}]
	if !c.ok || !kindOK {
		// release the lock early and read from the upstream.
		c.rw.RUnlock()
		resp, err := c.listResourcesFallback(ctx, req)
		return resp, trace.Wrap(err)

	}

	defer c.rw.RUnlock()

	resp, err := c.listResources(ctx, req)
	return resp, trace.Wrap(err)
}

func (c *Cache) listResourcesFallback(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/listResourcesFallback")
	defer span.End()

	if req.ResourceType != types.KindNode {
		out, err := c.Config.Presence.ListResources(ctx, req)
		return out, trace.Wrap(err)
	}

	cachedNodes, err := c.getNodesWithTTLCache(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers := types.Servers(cachedNodes)
	// Since TTLCaching falls back to retrieving all resources upfront, we also support
	// sorting.
	if err := servers.SortByCustom(req.SortBy); err != nil {
		return nil, trace.Wrap(err)
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
			return nil, trace.Wrap(err)
		}
		params.PredicateExpression = expression
	}

	resp, err := local.FakePaginate(servers.AsResources(), params)
	return resp, trace.Wrap(err)
}

func (c *Cache) listResources(ctx context.Context, req proto.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	_, span := c.Tracer.Start(ctx, "cache/listResources")
	defer span.End()

	filter := services.MatchResourceFilter{
		ResourceKind:   req.ResourceType,
		Labels:         req.Labels,
		SearchKeywords: req.SearchKeywords,
	}

	if req.PredicateExpression != "" {
		expression, err := services.NewResourceExpression(req.PredicateExpression)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		filter.PredicateExpression = expression
	}

	// Adjust page size, so it can't be empty.
	limit := int(req.Limit)
	if limit <= 0 {
		limit = defaults.DefaultChunkSize
	}

	switch req.ResourceType {
	case types.KindDatabaseServer:
		resp, err := buildListResourcesResponse(
			c.collections.dbServers.store.resources("name", req.StartKey, ""),
			limit,
			filter,
			types.DatabaseServer.CloneResource,
		)
		return resp, trace.Wrap(err)
	case types.KindDatabaseService:
		resp, err := buildListResourcesResponse(
			c.collections.dbServices.store.resources("name", req.StartKey, ""),
			limit,
			filter,
			func(d types.DatabaseService) types.ResourceWithLabels {
				return d.Clone()
			},
		)
		return resp, trace.Wrap(err)
	case types.KindAppServer:
		resp, err := buildListResourcesResponse(
			c.collections.appServers.store.resources("name", req.StartKey, ""),
			limit,
			filter,
			types.AppServer.CloneResource,
		)
		return resp, trace.Wrap(err)
	case types.KindNode:
		resp, err := buildListResourcesResponse(
			c.collections.nodes.store.resources("name", req.StartKey, ""),
			limit,
			filter,
			types.Server.CloneResource,
		)
		return resp, trace.Wrap(err)
	case types.KindWindowsDesktopService:
		resp, err := buildListResourcesResponse(
			c.collections.windowsDesktopServices.store.resources("name", req.StartKey, ""),
			limit,
			filter,
			func(d types.WindowsDesktopService) types.ResourceWithLabels {
				return d.Clone()
			},
		)
		return resp, trace.Wrap(err)
	case types.KindKubeServer:
		resp, err := buildListResourcesResponse(
			c.collections.kubeServers.store.resources("name", req.StartKey, ""),
			limit,
			filter,
			types.KubeServer.CloneResource,
		)
		return resp, trace.Wrap(err)
	case types.KindUserGroup:
		return nil, trace.NotImplemented("%s not implemented at ListResources", req.ResourceType)
	case types.KindIdentityCenterAccount:
		return nil, trace.NotImplemented("%s not implemented at ListResources", req.ResourceType)
	case types.KindIdentityCenterAccountAssignment:
		return nil, trace.NotImplemented("%s not implemented at ListResources", req.ResourceType)
	default:
		return nil, trace.NotImplemented("%s not implemented at ListResources", req.ResourceType)
	}
}

func buildListResourcesResponse[T types.ResourceWithLabels](resources iter.Seq[T], limit int, filter services.MatchResourceFilter, cloneFn func(T) types.ResourceWithLabels) (*types.ListResourcesResponse, error) {
	var resp types.ListResourcesResponse
	for r := range resources {
		rwl := any(r).(types.ResourceWithLabels)
		switch match, err := services.MatchResourceByFilters(rwl, filter, nil /* ignore dup matches */); {
		case err != nil:
			return nil, trace.Wrap(err)
		case match:
			if len(resp.Resources) == limit {
				resp.NextKey = backend.GetPaginationKey(r)
				return &resp, nil
			}

			resp.Resources = append(resp.Resources, cloneFn(r))
		}
	}

	return &resp, nil
}
