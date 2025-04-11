package cache

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func newAppCollection(p services.Apps, w types.WatchKind) (*collection[types.Application], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Apps")
	}

	return &collection[types.Application]{
		store: newStore(map[string]func(types.Application) string{
			"name": func(u types.Application) string {
				return u.GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Application, error) {
			return p.GetApps(ctx)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.Application {
			return &types.AppV3{
				Kind:    types.KindApp,
				Version: types.V3,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

// GetApps returns all application resources.
func (c *Cache) GetApps(ctx context.Context) ([]types.Application, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetApps")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.apps)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		out := make([]types.Application, 0, rg.store.len())
		for a := range rg.store.resources("name", "", "") {
			out = append(out, a.Copy())
		}

		return out, nil
	}

	apps, err := c.Config.Apps.GetApps(ctx)
	return apps, trace.Wrap(err)
}

// GetApp returns the specified application resource.
func (c *Cache) GetApp(ctx context.Context, name string) (types.Application, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetApp")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.apps)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		a, err := rg.store.get("name", name)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return a.Copy(), nil
	}

	apps, err := c.Config.Apps.GetApp(ctx, name)
	return apps, trace.Wrap(err)
}

func newAppServerCollection(p services.Presence, w types.WatchKind) (*collection[types.AppServer], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[types.AppServer]{
		store: newStore(map[string]func(types.AppServer) string{
			"name": func(u types.AppServer) string {
				return u.GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.AppServer, error) {
			return p.GetApplicationServers(ctx, defaults.Namespace)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.AppServer {
			return &types.AppServerV3{
				Kind:    types.KindAppServer,
				Version: types.V3,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
				Spec: types.AppServerSpecV3{
					HostID: hdr.Metadata.Description,
				},
			}
		},
		watch: w,
	}, nil
}

// GetApplicationServers returns all registered application servers.
func (c *Cache) GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetApplicationServers")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.appServers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		out := make([]types.AppServer, 0, rg.store.len())
		for as := range rg.store.resources("name", "", "") {
			out = append(out, as.Copy())
		}

		return out, nil
	}

	servers, err := c.Config.Presence.GetApplicationServers(ctx, namespace)
	return servers, trace.Wrap(err)
}
