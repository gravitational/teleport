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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

type windowsDesktopServiceIndex string

const windowsDesktopServiceNameIndex windowsDesktopServiceIndex = "name"

func newWindowsDesktopServiceCollection(p services.Presence, w types.WatchKind) (*collection[types.WindowsDesktopService, windowsDesktopServiceIndex], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[types.WindowsDesktopService, windowsDesktopServiceIndex]{
		store: newStore(
			types.WindowsDesktopService.Clone,
			map[windowsDesktopServiceIndex]func(types.WindowsDesktopService) string{
				windowsDesktopServiceNameIndex: types.WindowsDesktopService.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.WindowsDesktopService, error) {
			return p.GetWindowsDesktopServices(ctx)
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

// GetWindowsDesktopServices returns all registered Windows desktop services.
func (c *Cache) GetWindowsDesktopServices(ctx context.Context) ([]types.WindowsDesktopService, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetWindowsDesktopServices")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.windowsDesktopServices)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		services, err := c.Presence.GetWindowsDesktopServices(ctx)
		return services, trace.Wrap(err)
	}

	out := make([]types.WindowsDesktopService, 0, rg.store.len())
	for svc := range rg.store.resources(windowsDesktopServiceNameIndex, "", "") {
		out = append(out, svc.Clone())
	}

	return out, nil
}

// GetWindowsDesktopService returns a registered Windows desktop service by name.
func (c *Cache) GetWindowsDesktopService(ctx context.Context, name string) (types.WindowsDesktopService, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetWindowsDesktopService")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.windowsDesktopServices)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		service, err := c.Presence.GetWindowsDesktopService(ctx, name)
		return service, trace.Wrap(err)
	}

	svc, err := rg.store.get(windowsDesktopServiceNameIndex, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return svc.Clone(), nil
}

// ListWindowsDesktopServices returns all registered Windows desktop hosts.
func (c *Cache) ListWindowsDesktopServices(ctx context.Context, req types.ListWindowsDesktopServicesRequest) (*types.ListWindowsDesktopServicesResponse, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListWindowsDesktopServices")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.windowsDesktopServices)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		services, err := c.Config.WindowsDesktops.ListWindowsDesktopServices(ctx, req)
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

		resp.DesktopServices = append(resp.DesktopServices, svc.Clone())
	}

	return &resp, nil
}

type windowsDesktopIndex string

const windowsDesktopNameIndex windowsDesktopIndex = "name"

func newWindowsDesktopCollection(d services.WindowsDesktops, w types.WatchKind) (*collection[types.WindowsDesktop, windowsDesktopIndex], error) {
	if d == nil {
		return nil, trace.BadParameter("missing parameter Apps")
	}

	return &collection[types.WindowsDesktop, windowsDesktopIndex]{
		store: newStore(
			types.WindowsDesktop.Copy,
			map[windowsDesktopIndex]func(types.WindowsDesktop) string{
				windowsDesktopNameIndex: func(u types.WindowsDesktop) string {
					return u.GetHostID() + "/" + u.GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.WindowsDesktop, error) {
			return d.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
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

// GetWindowsDesktops returns all registered Windows desktop hosts.
func (c *Cache) GetWindowsDesktops(ctx context.Context, filter types.WindowsDesktopFilter) ([]types.WindowsDesktop, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetWindowsDesktops")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.windowsDesktops)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {

		desktops, err := c.Config.WindowsDesktops.GetWindowsDesktops(ctx, filter)
		return desktops, trace.Wrap(err)
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

// ListWindowsDesktops returns all registered Windows desktop hosts.
func (c *Cache) ListWindowsDesktops(ctx context.Context, req types.ListWindowsDesktopsRequest) (*types.ListWindowsDesktopsResponse, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListWindowsDesktops")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.windowsDesktops)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		desktops, err := c.Config.WindowsDesktops.ListWindowsDesktops(ctx, req)
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
				break
			}

			resp.Desktops = append(resp.Desktops, wd.Copy())
		}
	}

	return &resp, nil
}

type dynamicWindowsDesktopIndex string

const dynamicWindowsDesktopNameIndex dynamicWindowsDesktopIndex = "name"

func newDynamicWindowsDesktopCollection(upstream services.DynamicWindowsDesktops, w types.WatchKind) (*collection[types.DynamicWindowsDesktop, dynamicWindowsDesktopIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter DynamicWindowsDesktops")
	}

	return &collection[types.DynamicWindowsDesktop, dynamicWindowsDesktopIndex]{
		store: newStore(
			types.DynamicWindowsDesktop.Copy,
			map[dynamicWindowsDesktopIndex]func(types.DynamicWindowsDesktop) string{
				dynamicWindowsDesktopNameIndex: types.DynamicWindowsDesktop.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.DynamicWindowsDesktop, error) {
			var desktops []types.DynamicWindowsDesktop
			var next string
			for {
				d, token, err := upstream.ListDynamicWindowsDesktops(ctx, 0, next)
				if err != nil {
					return nil, err
				}
				desktops = append(desktops, d...)
				if token == "" {
					break
				}
				next = token
			}
			return desktops, nil
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

// GetDynamicWindowsDesktop returns registered dynamic Windows desktop by name.
func (c *Cache) GetDynamicWindowsDesktop(ctx context.Context, name string) (types.DynamicWindowsDesktop, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetDynamicWindowsDesktop")
	defer span.End()

	getter := genericGetter[types.DynamicWindowsDesktop, dynamicWindowsDesktopIndex]{
		cache:       c,
		collection:  c.collections.dynamicWindowsDesktops,
		index:       dynamicWindowsDesktopNameIndex,
		upstreamGet: c.Config.DynamicWindowsDesktops.GetDynamicWindowsDesktop,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

// ListDynamicWindowsDesktops returns all registered dynamic Windows desktop.
func (c *Cache) ListDynamicWindowsDesktops(ctx context.Context, pageSize int, nextPage string) ([]types.DynamicWindowsDesktop, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListDynamicWindowsDesktops")
	defer span.End()

	lister := genericLister[types.DynamicWindowsDesktop, dynamicWindowsDesktopIndex]{
		cache:        c,
		collection:   c.collections.dynamicWindowsDesktops,
		index:        dynamicWindowsDesktopNameIndex,
		upstreamList: c.Config.DynamicWindowsDesktops.ListDynamicWindowsDesktops,
		nextToken: func(dwd types.DynamicWindowsDesktop) string {
			return dwd.GetMetadata().Name
		},
	}
	out, next, err := lister.list(ctx, pageSize, nextPage)
	return out, next, trace.Wrap(err)
}
