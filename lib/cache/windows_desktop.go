package cache

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
)

func newWindowsDesktopServiceCollection(p services.Presence, w types.WatchKind) (*collection[types.WindowsDesktopService], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[types.WindowsDesktopService]{
		store: newStore(map[string]func(types.WindowsDesktopService) string{
			"name": func(u types.WindowsDesktopService) string {
				return u.GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.WindowsDesktopService, error) {
			return p.GetWindowsDesktopServices(ctx)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.WindowsDesktopService {
			return &types.WindowsDesktopServiceV3{
				ResourceHeader: types.ResourceHeader{
					Kind:    types.KindWindowsDesktopService,
					Version: types.V3,
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

	if rg.ReadCache() {
		out := make([]types.WindowsDesktopService, 0, rg.store.len())
		for svc := range rg.store.resources("name", "", "") {
			out = append(out, svc.Clone())
		}

		return out, nil
	}

	services, err := c.Presence.GetWindowsDesktopServices(ctx)
	return services, trace.Wrap(err)
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

	if rg.ReadCache() {
		svc, err := rg.store.get("name", name)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return svc.Clone(), nil
	}

	service, err := c.Presence.GetWindowsDesktopService(ctx, name)
	return service, trace.Wrap(err)
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
	for svc := range rg.store.resources("name", req.StartKey, "") {
		if len(resp.DesktopServices) == pageSize {
			resp.NextKey = backend.GetPaginationKey(svc)
			break
		}

		resp.DesktopServices = append(resp.DesktopServices, svc.Clone())
	}

	return &resp, nil
}

func newWindowsDesktopCollection(d services.WindowsDesktops, w types.WatchKind) (*collection[types.WindowsDesktop], error) {
	if d == nil {
		return nil, trace.BadParameter("missing parameter Apps")
	}

	return &collection[types.WindowsDesktop]{
		store: newStore(map[string]func(types.WindowsDesktop) string{
			"name": func(u types.WindowsDesktop) string {
				return u.GetHostID() + "/" + u.GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.WindowsDesktop, error) {
			return d.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{})
		},
		headerTransform: func(hdr *types.ResourceHeader) types.WindowsDesktop {
			return &types.WindowsDesktopV3{
				ResourceHeader: types.ResourceHeader{
					Kind:    types.KindWindowsDesktop,
					Version: types.V3,
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

	if rg.ReadCache() {
		out := make([]types.WindowsDesktop, 0, rg.store.len())
		for wd := range rg.store.resources("name", "", "") {
			if !filter.Match(wd) {
				continue
			}

			out = append(out, wd.Copy())
		}

		return out, nil
	}

	desktops, err := c.Config.WindowsDesktops.GetWindowsDesktops(ctx, filter)
	return desktops, trace.Wrap(err)
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
	for wd := range rg.store.resources("name", req.StartKey, "") {
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

// GetDynamicWindowsDesktop returns registered dynamic Windows desktop by name.
func (c *Cache) GetDynamicWindowsDesktop(ctx context.Context, name string) (types.DynamicWindowsDesktop, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetDynamicWindowsDesktop")
	defer span.End()

	rg, err := readLegacyCollectionCache(c, c.legacyCacheCollections.dynamicWindowsDesktops)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.GetDynamicWindowsDesktop(ctx, name)
}

// ListDynamicWindowsDesktops returns all registered dynamic Windows desktop.
func (c *Cache) ListDynamicWindowsDesktops(ctx context.Context, pageSize int, nextPage string) ([]types.DynamicWindowsDesktop, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListDynamicWindowsDesktops")
	defer span.End()

	rg, err := readLegacyCollectionCache(c, c.legacyCacheCollections.dynamicWindowsDesktops)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer rg.Release()
	return rg.reader.ListDynamicWindowsDesktops(ctx, pageSize, nextPage)
}
