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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type appIndex string

const appNameIndex appIndex = "name"

func newAppCollection(upstream services.Applications, w types.WatchKind) (*collection[types.Application, appIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Applications")
	}

	return &collection[types.Application, appIndex]{
		store: newStore(
			types.KindApp,
			func(a types.Application) types.Application {
				return a.Copy()
			},
			map[appIndex]func(types.Application) string{
				appNameIndex: types.Application.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Application, error) {
			out, err := stream.Collect(upstream.Apps(ctx, "", ""))
			// TODO(tross): DELETE IN v21.0.0
			if trace.IsNotImplemented(err) {
				apps, err := upstream.GetApps(ctx)
				return apps, trace.Wrap(err)
			}
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.Application {
			return &types.AppV3{
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

// Apps returns application resources within the range [start, end).
func (c *Cache) Apps(ctx context.Context, start, end string) iter.Seq2[types.Application, error] {
	ctx, span := c.Tracer.Start(ctx, "cache/Apps")
	defer span.End()

	return func(yield func(types.Application, error) bool) {
		rg, err := acquireReadGuard(c, c.collections.apps)
		if err != nil {
			yield(nil, err)
			return
		}
		defer rg.Release()

		if rg.ReadCache() {
			for a := range rg.store.resources(appNameIndex, start, end) {
				if !yield(a, nil) {
					return
				}
			}
			return
		}

		// Release the read guard early since all future reads will be
		// performed against the upstream.
		rg.Release()

		for app, err := range c.Config.Apps.Apps(ctx, start, end) {
			if err != nil {
				// TODO(tross): DELETE IN v21.0.0
				if trace.IsNotImplemented(err) {
					apps, err := c.Config.Apps.GetApps(ctx)
					if err != nil {
						yield(nil, err)
						return
					}

					for _, app := range apps {
						if !yield(app, nil) {
							return
						}
					}

					return
				}

				yield(nil, err)
				return
			}

			if !yield(app, nil) {
				return
			}
		}
	}
}

// ListApps returns a page of application resources.
func (c *Cache) ListApps(ctx context.Context, limit int, startKey string) ([]types.Application, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListApps")
	defer span.End()

	lister := genericLister[types.Application, appIndex]{
		cache:        c,
		collection:   c.collections.apps,
		index:        appNameIndex,
		upstreamList: c.Config.Apps.ListApps,
		nextToken: func(a types.Application) string {
			return a.GetMetadata().Name
		},
	}
	out, next, err := lister.list(ctx, limit, startKey)
	return out, next, trace.Wrap(err)
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

	if !rg.ReadCache() {
		apps, err := c.Config.Apps.GetApps(ctx)
		return apps, trace.Wrap(err)
	}

	out := make([]types.Application, 0, rg.store.len())
	for a := range rg.store.resources(appNameIndex, "", "") {
		out = append(out, a.Copy())
	}

	return out, nil
}

// GetApp returns the specified application resource.
func (c *Cache) GetApp(ctx context.Context, name string) (types.Application, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetApp")
	defer span.End()

	getter := genericGetter[types.Application, appIndex]{
		cache:       c,
		collection:  c.collections.apps,
		index:       appNameIndex,
		upstreamGet: c.Config.Apps.GetApp,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

type appServerIndex string

const appServerNameIndex appServerIndex = "name"

func newAppServerCollection(p services.Presence, w types.WatchKind) (*collection[types.AppServer, appServerIndex], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[types.AppServer, appServerIndex]{
		store: newStore(
			types.KindAppServer,
			types.AppServer.Copy,
			map[appServerIndex]func(types.AppServer) string{
				appServerNameIndex: func(u types.AppServer) string {
					return u.GetHostID() + "/" + u.GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.AppServer, error) {
			return p.GetApplicationServers(ctx, defaults.Namespace)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.AppServer {
			return &types.AppServerV3{
				Kind:    hdr.Kind,
				Version: hdr.Version,
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
		for as := range rg.store.resources(appServerNameIndex, "", "") {
			out = append(out, as.Copy())
		}

		return out, nil
	}

	servers, err := c.Config.Presence.GetApplicationServers(ctx, namespace)
	return servers, trace.Wrap(err)
}
