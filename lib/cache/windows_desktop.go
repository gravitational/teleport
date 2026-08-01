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

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cache/internal"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type windowsDesktopServiceIndex string

const windowsDesktopServiceNameIndex windowsDesktopServiceIndex = "name"

func newWindowsDesktopServiceCollection(upstream services.Presence, w types.WatchKind) (*collection[types.WindowsDesktopService, windowsDesktopServiceIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[types.WindowsDesktopService, windowsDesktopServiceIndex]{
		store: newStore(
			types.KindWindowsDesktopService,
			types.WindowsDesktopService.Clone,
			map[windowsDesktopServiceIndex]func(types.WindowsDesktopService) string{
				windowsDesktopServiceNameIndex: types.WindowsDesktopService.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.WindowsDesktopService, error) {
			resources, err := client.GetResourcesWithFilters(ctx, upstream, proto.ListResourcesRequest{ResourceType: types.KindWindowsDesktopService})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			desktopSvcs := make([]types.WindowsDesktopService, 0, len(resources))
			for _, resource := range resources {
				desktopSvc, ok := resource.(types.WindowsDesktopService)
				if !ok {
					return nil, trace.BadParameter("unexpected resource %T", resource)
				}
				desktopSvcs = append(desktopSvcs, desktopSvc)
			}

			return desktopSvcs, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.WindowsDesktopService {
			return &types.WindowsDesktopServiceV3{
				ResourceHeader: types.ResourceHeader{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: types.Metadata{
						Name: hdr.Metadata.Name,
					},
				},
			}
		},
		watch: w,
	}, nil
}

// windowsDesktopServiceCollection provides read access to cached Windows
// desktop services. Its exported methods are promoted onto every topology
// cache that embeds it; the reads are implemented exactly once here. It is a
// stateless value assembled inline by each of its consumers so that no
// shared scaffolding couples their lifetimes.
type windowsDesktopServiceCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.Presence
	// upstreamDesktops serves the upstream fallback for
	// ListWindowsDesktopServices, which lives on the WindowsDesktops service
	// rather than on Presence.
	upstreamDesktops services.WindowsDesktops
	col              *collection[types.WindowsDesktopService, windowsDesktopServiceIndex]
}

// GetWindowsDesktopServices returns all registered Windows desktop services.
func (c windowsDesktopServiceCollection) GetWindowsDesktopServices(ctx context.Context) ([]types.WindowsDesktopService, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetWindowsDesktopServices")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		services, err := c.upstream.GetWindowsDesktopServices(ctx)
		return services, trace.Wrap(err)
	}

	out := make([]types.WindowsDesktopService, 0, rg.store.len())
	for svc := range rg.store.resources(windowsDesktopServiceNameIndex, "", "") {
		out = append(out, svc.Clone())
	}

	return out, nil
}

// GetWindowsDesktopServices returns all registered Windows desktop services.
func (c *Cache) GetWindowsDesktopServices(ctx context.Context) ([]types.WindowsDesktopService, error) {
	return windowsDesktopServiceCollection{
		engine:           c.engine,
		tracer:           c.Tracer,
		upstream:         c.Config.Presence,
		upstreamDesktops: c.Config.WindowsDesktops,
		col:              c.collections.windowsDesktopServices,
	}.GetWindowsDesktopServices(ctx)
}

// GetWindowsDesktopService returns a registered Windows desktop service by name.
func (c windowsDesktopServiceCollection) GetWindowsDesktopService(ctx context.Context, name string) (types.WindowsDesktopService, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetWindowsDesktopService")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		service, err := c.upstream.GetWindowsDesktopService(ctx, name)
		return service, trace.Wrap(err)
	}

	svc, err := rg.store.get(windowsDesktopServiceNameIndex, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return svc.Clone(), nil
}

// GetWindowsDesktopService returns a registered Windows desktop service by name.
func (c *Cache) GetWindowsDesktopService(ctx context.Context, name string) (types.WindowsDesktopService, error) {
	return windowsDesktopServiceCollection{
		engine:           c.engine,
		tracer:           c.Tracer,
		upstream:         c.Config.Presence,
		upstreamDesktops: c.Config.WindowsDesktops,
		col:              c.collections.windowsDesktopServices,
	}.GetWindowsDesktopService(ctx, name)
}

// ListWindowsDesktopServices returns all registered Windows desktop hosts.
func (c windowsDesktopServiceCollection) ListWindowsDesktopServices(ctx context.Context, req types.ListWindowsDesktopServicesRequest) (*types.ListWindowsDesktopServicesResponse, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListWindowsDesktopServices")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		services, err := c.upstreamDesktops.ListWindowsDesktopServices(ctx, req)
		return services, trace.Wrap(err)
	}

	filter := services.MatchResourceFilter{
		ResourceKind:   types.KindWindowsDesktopService,
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

	// Adjust page size, so it can't be too large or small.
	pageSize := req.Limit
	if pageSize <= 0 || pageSize > defaults.DefaultChunkSize {
		pageSize = defaults.DefaultChunkSize
	}

	var resp types.ListWindowsDesktopServicesResponse
	for svc := range rg.store.resources(windowsDesktopServiceNameIndex, req.StartKey, "") {
		if len(resp.DesktopServices) == pageSize {
			resp.NextKey = backend.GetPaginationKey(svc)
			break
		}

		switch match, err := services.MatchResourceByFilters(svc, filter, nil /* ignore dup matches */); {
		case err != nil:
			return nil, trace.Wrap(err)
		case match:
			resp.DesktopServices = append(resp.DesktopServices, svc.Clone())
		}
	}

	return &resp, nil
}

// ListWindowsDesktopServices returns all registered Windows desktop hosts.
func (c *Cache) ListWindowsDesktopServices(ctx context.Context, req types.ListWindowsDesktopServicesRequest) (*types.ListWindowsDesktopServicesResponse, error) {
	return windowsDesktopServiceCollection{
		engine:           c.engine,
		tracer:           c.Tracer,
		upstream:         c.Config.Presence,
		upstreamDesktops: c.Config.WindowsDesktops,
		col:              c.collections.windowsDesktopServices,
	}.ListWindowsDesktopServices(ctx, req)
}

type windowsDesktopIndex string

const windowsDesktopNameIndex windowsDesktopIndex = "name"

func newWindowsDesktopCollection(upstream services.WindowsDesktops, w types.WatchKind) (*collection[types.WindowsDesktop, windowsDesktopIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter WindowsDesktops")
	}

	return &collection[types.WindowsDesktop, windowsDesktopIndex]{
		store: newStore(
			types.KindWindowsDesktop,
			types.WindowsDesktop.Copy,
			map[windowsDesktopIndex]func(types.WindowsDesktop) string{
				windowsDesktopNameIndex: func(u types.WindowsDesktop) string {
					return u.GetHostID() + "/" + u.GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.WindowsDesktop, error) {
			// TODO(tross): DELETE in V21.0.0  replace by regular clientutils.Resources
			out, err := clientutils.CollectWithFallback(
				ctx,
				func(ctx context.Context, limit int, start string) ([]types.WindowsDesktop, string, error) {
					resp, err := upstream.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
						Limit:    limit,
						StartKey: start,
					})
					if err != nil {
						return nil, "", trace.Wrap(err)
					}
					return resp.Desktops, resp.NextKey, nil
				},
				func(ctx context.Context) ([]types.WindowsDesktop, error) {
					return upstream.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
				},
			)

			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.WindowsDesktop {
			return &types.WindowsDesktopV3{
				ResourceHeader: types.ResourceHeader{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: types.Metadata{
						Name: hdr.Metadata.Name,
					},
				},
				Spec: types.WindowsDesktopSpecV3{
					HostID: hdr.Metadata.Description,
				},
			}
		},
		watch: w,
	}, nil
}

// windowsDesktopCollection provides read access to cached Windows desktops.
// Its exported methods are promoted onto every topology cache that embeds
// it; the reads are implemented exactly once here. It is a stateless value
// assembled inline by each of its consumers so that no shared scaffolding
// couples their lifetimes.
type windowsDesktopCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.WindowsDesktops
	col      *collection[types.WindowsDesktop, windowsDesktopIndex]
}

// GetWindowsDesktops returns all registered Windows desktop hosts.
func (c windowsDesktopCollection) GetWindowsDesktops(ctx context.Context, filter types.WindowsDesktopFilter) ([]types.WindowsDesktop, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetWindowsDesktops")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		desktops, err := c.upstream.GetWindowsDesktops(ctx, filter)
		return desktops, trace.Wrap(err)
	}

	if filter.HostID != "" && filter.Name != "" {
		desktop, err := rg.store.get(windowsDesktopNameIndex, filter.HostID+"/"+filter.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if !filter.Match(desktop) {
			return []types.WindowsDesktop{}, nil
		}
		return []types.WindowsDesktop{desktop}, nil
	}

	out := make([]types.WindowsDesktop, 0, rg.store.len())
	for wd := range rg.store.resources(windowsDesktopNameIndex, "", "") {
		if !filter.Match(wd) {
			continue
		}

		out = append(out, wd.Copy())
	}

	return out, nil
}

// GetWindowsDesktops returns all registered Windows desktop hosts.
func (c *Cache) GetWindowsDesktops(ctx context.Context, filter types.WindowsDesktopFilter) ([]types.WindowsDesktop, error) {
	return windowsDesktopCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.WindowsDesktops,
		col:      c.collections.windowsDesktops,
	}.GetWindowsDesktops(ctx, filter)
}

// ListWindowsDesktops returns all registered Windows desktop hosts.
func (c windowsDesktopCollection) ListWindowsDesktops(ctx context.Context, req types.ListWindowsDesktopsRequest) (*types.ListWindowsDesktopsResponse, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListWindowsDesktops")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		desktops, err := c.upstream.ListWindowsDesktops(ctx, req)
		return desktops, trace.Wrap(err)
	}

	filter := services.MatchResourceFilter{
		ResourceKind:   types.KindWindowsDesktop,
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

	// Adjust page size, so it can't be too large or small.
	pageSize := req.Limit
	if pageSize <= 0 || pageSize > defaults.DefaultChunkSize {
		pageSize = defaults.DefaultChunkSize
	}

	var resp types.ListWindowsDesktopsResponse

	for wd := range rg.store.resources(windowsDesktopNameIndex, req.StartKey, "") {
		if !req.WindowsDesktopFilter.Match(wd) {
			continue
		}

		switch match, err := services.MatchResourceByFilters(wd, filter, nil /* ignore dup matches */); {
		case err != nil:
			return nil, trace.Wrap(err)
		case match:
			if len(resp.Desktops) == pageSize {
				resp.NextKey = backend.GetPaginationKey(wd)
				return &resp, nil
			}
			resp.Desktops = append(resp.Desktops, wd.Copy())
		}
	}

	return &resp, nil
}

// ListWindowsDesktops returns all registered Windows desktop hosts.
func (c *Cache) ListWindowsDesktops(ctx context.Context, req types.ListWindowsDesktopsRequest) (*types.ListWindowsDesktopsResponse, error) {
	return windowsDesktopCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.WindowsDesktops,
		col:      c.collections.windowsDesktops,
	}.ListWindowsDesktops(ctx, req)
}

type dynamicWindowsDesktopIndex string

const dynamicWindowsDesktopNameIndex dynamicWindowsDesktopIndex = "name"

func newDynamicWindowsDesktopCollection(upstream services.DynamicWindowsDesktops, w types.WatchKind) (*collection[types.DynamicWindowsDesktop, dynamicWindowsDesktopIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter DynamicWindowsDesktops")
	}

	return &collection[types.DynamicWindowsDesktop, dynamicWindowsDesktopIndex]{
		store: newStore(
			types.KindDynamicWindowsDesktop,
			types.DynamicWindowsDesktop.Copy,
			map[dynamicWindowsDesktopIndex]func(types.DynamicWindowsDesktop) string{
				dynamicWindowsDesktopNameIndex: types.DynamicWindowsDesktop.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.DynamicWindowsDesktop, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListDynamicWindowsDesktops))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.DynamicWindowsDesktop {
			return &types.DynamicWindowsDesktopV1{
				ResourceHeader: types.ResourceHeader{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: types.Metadata{
						Name: hdr.Metadata.Name,
					},
				},
			}
		},
		watch: w,
	}, nil
}

// dynamicWindowsDesktopCollection provides read access to cached dynamic
// Windows desktops. Its exported methods are promoted onto every topology
// cache that embeds it; the reads are implemented exactly once here. It is a
// stateless value assembled inline by each of its consumers so that no
// shared scaffolding couples their lifetimes.
type dynamicWindowsDesktopCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.DynamicWindowsDesktops
	col      *collection[types.DynamicWindowsDesktop, dynamicWindowsDesktopIndex]
}

// GetDynamicWindowsDesktop returns registered dynamic Windows desktop by name.
func (c dynamicWindowsDesktopCollection) GetDynamicWindowsDesktop(ctx context.Context, name string) (types.DynamicWindowsDesktop, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetDynamicWindowsDesktop")
	defer span.End()

	getter := genericGetter[types.DynamicWindowsDesktop, dynamicWindowsDesktopIndex]{
		engine:      c.engine,
		collection:  c.col,
		index:       dynamicWindowsDesktopNameIndex,
		upstreamGet: c.upstream.GetDynamicWindowsDesktop,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

// GetDynamicWindowsDesktop returns registered dynamic Windows desktop by name.
func (c *Cache) GetDynamicWindowsDesktop(ctx context.Context, name string) (types.DynamicWindowsDesktop, error) {
	return dynamicWindowsDesktopCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.DynamicWindowsDesktops,
		col:      c.collections.dynamicWindowsDesktops,
	}.GetDynamicWindowsDesktop(ctx, name)
}

// ListDynamicWindowsDesktops returns all registered dynamic Windows desktop.
func (c dynamicWindowsDesktopCollection) ListDynamicWindowsDesktops(ctx context.Context, pageSize int, nextPage string) ([]types.DynamicWindowsDesktop, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListDynamicWindowsDesktops")
	defer span.End()

	lister := genericLister[types.DynamicWindowsDesktop, dynamicWindowsDesktopIndex]{
		engine:       c.engine,
		collection:   c.col,
		index:        dynamicWindowsDesktopNameIndex,
		upstreamList: c.upstream.ListDynamicWindowsDesktops,
		nextToken: func(dwd types.DynamicWindowsDesktop) string {
			return dwd.GetMetadata().Name
		},
	}
	out, next, err := lister.list(ctx, pageSize, nextPage)
	return out, next, trace.Wrap(err)
}

// ListDynamicWindowsDesktops returns all registered dynamic Windows desktop.
func (c *Cache) ListDynamicWindowsDesktops(ctx context.Context, pageSize int, nextPage string) ([]types.DynamicWindowsDesktop, string, error) {
	return dynamicWindowsDesktopCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.DynamicWindowsDesktops,
		col:      c.collections.dynamicWindowsDesktops,
	}.ListDynamicWindowsDesktops(ctx, pageSize, nextPage)
}
