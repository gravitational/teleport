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
	"strings"

	"github.com/gravitational/trace"
	"rsc.io/ordered"

	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	logutils "github.com/gravitational/teleport/lib/utils/log"
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
			// TODO(tross): DELETE IN v21.0.0 replace by regular clientutils.Resources
			out, err := clientutils.CollectWithFallback(ctx, upstream.ListApps, upstream.GetApps)
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
	lister := genericLister[types.Application, appIndex]{
		cache:        c,
		collection:   c.collections.apps,
		index:        appNameIndex,
		upstreamList: c.Config.Apps.ListApps,
		nextToken:    types.Application.GetName,
		// TODO(tross): DELETE IN v21.0.0
		fallbackGetter: c.Config.Apps.GetApps,
	}

	return func(yield func(types.Application, error) bool) {
		ctx, span := c.Tracer.Start(ctx, "cache/Apps")
		defer span.End()

		for app, err := range lister.RangeWithFallback(ctx, start, end) {
			if !yield(app, err) {
				return
			}

			if err != nil {
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
const appServerAppNameIndex appServerIndex = "app_name"

func appServerByAppNameKey(s types.AppServer) string {
	// Delete events deliver header only resources with a nil App. This returns
	// "" so the secondary index lookup is a no-op. The primary index deletion
	// removes the entry from all indexes.
	app := s.GetApp()
	if app == nil {
		return ""
	}
	return string(ordered.Encode(app.GetName(), s.GetHostID(), s.GetName()))
}

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
				appServerAppNameIndex: appServerByAppNameKey,
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

// RangeApplicationServersWithName returns an iterator over application servers for a given app name.
func (c *Cache) RangeApplicationServersWithName(ctx context.Context, appName string) iter.Seq2[types.AppServer, error] {
	if appName == "" {
		return stream.Fail[types.AppServer](trace.BadParameter("missing application name"))
	}

	return func(yield func(types.AppServer, error) bool) {
		ctx, span := c.Tracer.Start(ctx, "cache/RangeApplicationServersWithName")
		defer span.End()

		upstreamListFn := func(ctx context.Context, pageSize int, startToken string) ([]types.AppServer, string, error) {
			var tokenAppName string
			rest, err := ordered.DecodePrefix([]byte(startToken), &tokenAppName)
			if err != nil {
				return nil, "", trace.Wrap(err)
			}

			// Verify that the token's app name matches the requested app name.
			// This ensures that if the token is malformed or belongs to a different
			// app, we don't return incorrect results.
			if tokenAppName != appName {
				return nil, "", trace.BadParameter("pagination token does not match the requested application name")
			}

			backendKey := ""
			if len(rest) > 0 {
				var hostID, serverName string
				if err := ordered.Decode(rest, &hostID, &serverName); err != nil {
					return nil, "", trace.Wrap(err)
				}
				backendKey = hostID + "/" + serverName
			}

			resp, err := c.Config.Presence.ListResources(ctx, clientproto.ListResourcesRequest{
				ResourceType: types.KindAppServer,
				Namespace:    defaults.Namespace,
				Limit:        int32(pageSize),
				StartKey:     backendKey,
			})
			if err != nil {
				return nil, "", trace.Wrap(err)
			}

			var page []types.AppServer
			for _, r := range resp.Resources {
				server, ok := r.(types.AppServer)
				if !ok {
					c.Logger.WarnContext(ctx, "expected AppServer but received unexpected type", "resource_type", logutils.TypeAttr(r))
					continue
				}
				if app := server.GetApp(); app != nil && app.GetName() == appName {
					page = append(page, server)
				}
			}

			next := ""
			if resp.NextKey != "" {
				hostID, serverName, ok := strings.Cut(resp.NextKey, backend.SeparatorString)
				if !ok {
					return nil, "", trace.BadParameter("invalid pagination token: %q", resp.NextKey)
				}
				next = string(ordered.Encode(appName, hostID, serverName))
			}
			return page, next, nil
		}

		lister := genericLister[types.AppServer, appServerIndex]{
			cache:           c,
			collection:      c.collections.appServers,
			index:           appServerAppNameIndex,
			nextToken:       appServerByAppNameKey,
			defaultPageSize: defaults.DefaultChunkSize,
			upstreamList:    upstreamListFn,
		}

		start := string(ordered.Encode(appName))
		end := string(ordered.Encode(appName, ordered.Inf))
		for item, err := range lister.Range(ctx, start, end) {
			if !yield(item, err) {
				return
			}
		}
	}
}
