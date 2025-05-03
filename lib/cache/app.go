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
	"github.com/gravitational/teleport/lib/services"
)

type appIndex string

const appNameIndex appIndex = "name"

func newAppCollection(p services.Apps, w types.WatchKind) (*collection[types.Application, appIndex], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Apps")
	}

	return &collection[types.Application, appIndex]{
		store: newStore(
			func(a types.Application) types.Application {
				return a.Copy()
			},
			map[appIndex]func(types.Application) string{
				appNameIndex: types.Application.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Application, error) {
			apps, err := p.GetApps(ctx)
			return apps, trace.Wrap(err)
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
