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
	"log/slog"

	"github.com/gravitational/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"rsc.io/ordered"

	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/cache/internal"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/readonly"
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

// appCollection provides read access to cached applications. Its exported
// methods are promoted onto every topology cache that embeds it; the reads
// are implemented exactly once here. It is a stateless value assembled inline
// by each of its consumers so that no shared scaffolding couples their
// lifetimes.
type appCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.Applications
	col      *collection[types.Application, appIndex]
}

// Apps returns application resources within the range [start, end).
func (c appCollection) Apps(ctx context.Context, start, end string) iter.Seq2[types.Application, error] {
	lister := genericLister[types.Application, appIndex]{
		engine:       c.engine,
		collection:   c.col,
		index:        appNameIndex,
		upstreamList: c.upstream.ListApps,
		nextToken:    types.Application.GetName,
		// TODO(tross): DELETE IN v21.0.0
		fallbackGetter: c.upstream.GetApps,
	}

	return func(yield func(types.Application, error) bool) {
		ctx, span := c.tracer.Start(ctx, "cache/Apps")
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

// Apps returns application resources within the range [start, end).
func (c *Cache) Apps(ctx context.Context, start, end string) iter.Seq2[types.Application, error] {
	return appCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Apps,
		col:      c.collections.apps,
	}.Apps(ctx, start, end)
}

// ListApps returns a page of application resources.
func (c appCollection) ListApps(ctx context.Context, limit int, startKey string) ([]types.Application, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListApps")
	defer span.End()

	lister := genericLister[types.Application, appIndex]{
		engine:       c.engine,
		collection:   c.col,
		index:        appNameIndex,
		upstreamList: c.upstream.ListApps,
		nextToken: func(a types.Application) string {
			return a.GetMetadata().Name
		},
	}
	out, next, err := lister.list(ctx, limit, startKey)
	return out, next, trace.Wrap(err)
}

// ListApps returns a page of application resources.
func (c *Cache) ListApps(ctx context.Context, limit int, startKey string) ([]types.Application, string, error) {
	return appCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Apps,
		col:      c.collections.apps,
	}.ListApps(ctx, limit, startKey)
}

// GetApps returns all application resources.
func (c appCollection) GetApps(ctx context.Context) ([]types.Application, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetApps")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		apps, err := c.upstream.GetApps(ctx)
		return apps, trace.Wrap(err)
	}

	out := make([]types.Application, 0, rg.store.len())
	for a := range rg.store.resources(appNameIndex, "", "") {
		out = append(out, a.Copy())
	}

	return out, nil
}

// GetApps returns all application resources.
func (c *Cache) GetApps(ctx context.Context) ([]types.Application, error) {
	return appCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Apps,
		col:      c.collections.apps,
	}.GetApps(ctx)
}

// GetApp returns the specified application resource.
func (c appCollection) GetApp(ctx context.Context, name string) (types.Application, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetApp")
	defer span.End()

	getter := genericGetter[types.Application, appIndex]{
		engine:      c.engine,
		collection:  c.col,
		index:       appNameIndex,
		upstreamGet: c.upstream.GetApp,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

// GetApp returns the specified application resource.
func (c *Cache) GetApp(ctx context.Context, name string) (types.Application, error) {
	return appCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Apps,
		col:      c.collections.apps,
	}.GetApp(ctx, name)
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
	// The second component is the scope-aware resource cursor, so within each
	// app name the in-memory ordering matches the backend listing order
	// (unscoped first, then scoped) and index keys are interchangeable with
	// the fallback pagination tokens.
	return string(ordered.Encode(app.GetName(), services.GetCursorForAppServer(s)))
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
				appServerNameIndex:    services.GetCursorForAppServer,
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

// appServerCollection provides read access to cached application servers.
// Its exported methods are promoted onto every topology cache that embeds it;
// the reads are implemented exactly once here. It is a stateless value
// assembled inline by each of its consumers so that no shared scaffolding
// couples their lifetimes.
type appServerCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	logger   *slog.Logger
	upstream services.Presence
	col      *collection[types.AppServer, appServerIndex]
}

// GetApplicationServers returns all registered application servers.
func (c appServerCollection) GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetApplicationServers")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
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

	servers, err := c.upstream.GetApplicationServers(ctx, namespace)
	return servers, trace.Wrap(err)
}

// GetApplicationServers returns all registered application servers.
func (c *Cache) GetApplicationServers(ctx context.Context, namespace string) ([]types.AppServer, error) {
	return appServerCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		logger:   c.Logger,
		upstream: c.Config.Presence,
		col:      c.collections.appServers,
	}.GetApplicationServers(ctx, namespace)
}

// RangeReadonlyApplicationServers returns read-only views of the application
// server resources within the range [start, end), where the bounds are
// resource cursors as produced by services.GetCursorForAppServer. The yielded
// values are shared with the cache: they must not be mutated or retained
// beyond the iteration — callers keep a match by calling Copy on it.
//
// The healthy read path is served from a single consistent snapshot of the
// cache captured when iteration begins: it holds no locks, does not block
// cache updates, and observes neither missed nor duplicated servers
// regardless of concurrent changes. This is the primitive that replaces
// maintaining a secondary materialized view via services.AppServersWatcher.
func (c appServerCollection) RangeReadonlyApplicationServers(ctx context.Context, start, end string) iter.Seq2[readonly.AppServer, error] {
	return func(yield func(readonly.AppServer, error) bool) {
		ctx, span := c.tracer.Start(ctx, "cache/RangeReadonlyApplicationServers")
		defer span.End()

		rg, err := acquireGuard(c.engine, c.col)
		if err != nil {
			yield(nil, trace.Wrap(err))
			return
		}
		defer rg.Release()

		if !rg.ReadCache() {
			servers, err := c.upstream.GetApplicationServers(ctx, defaults.Namespace)
			if err != nil {
				yield(nil, trace.Wrap(err))
				return
			}

			for _, server := range servers {
				cursor := services.GetCursorForAppServer(server)
				if cursor < start {
					continue
				}
				if end != "" && cursor >= end {
					continue
				}
				if !yield(server, nil) {
					return
				}
			}
			return
		}

		for server := range rg.store.resources(appServerNameIndex, start, end) {
			if !yield(server, nil) {
				return
			}
		}
	}
}

// RangeReadonlyApplicationServers returns read-only views of the application
// server resources within the range [start, end), where the bounds are
// resource cursors as produced by services.GetCursorForAppServer. The yielded
// values are shared with the cache: they must not be mutated or retained
// beyond the iteration — callers keep a match by calling Copy on it.
func (c *Cache) RangeReadonlyApplicationServers(ctx context.Context, start, end string) iter.Seq2[readonly.AppServer, error] {
	return appServerCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		logger:   c.Logger,
		upstream: c.Config.Presence,
		col:      c.collections.appServers,
	}.RangeReadonlyApplicationServers(ctx, start, end)
}

// RangeApplicationServersWithName returns an iterator over application servers for a given app name.
func (c appServerCollection) RangeApplicationServersWithName(ctx context.Context, appName string) iter.Seq2[types.AppServer, error] {
	if appName == "" {
		return stream.Fail[types.AppServer](trace.BadParameter("missing application name"))
	}

	return func(yield func(types.AppServer, error) bool) {
		ctx, span := c.tracer.Start(ctx, "cache/RangeApplicationServersWithName")
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

			// The remainder of the token is the scope aware resource
			// cursor
			startKey := ""
			if len(rest) > 0 {
				if err := ordered.Decode(rest, &startKey); err != nil {
					return nil, "", trace.Wrap(err)
				}
			}

			resp, err := c.upstream.ListResources(ctx, clientproto.ListResourcesRequest{
				ResourceType: types.KindAppServer,
				Namespace:    defaults.Namespace,
				Limit:        int32(pageSize),
				StartKey:     startKey,
			})
			if err != nil {
				return nil, "", trace.Wrap(err)
			}

			var page []types.AppServer
			for _, r := range resp.Resources {
				server, ok := r.(types.AppServer)
				if !ok {
					c.logger.WarnContext(ctx, "expected AppServer but received unexpected type", "resource_type", logutils.TypeAttr(r))
					continue
				}
				if app := server.GetApp(); app != nil && app.GetName() == appName {
					page = append(page, server)
				}
			}

			next := ""
			if resp.NextKey != "" {
				next = string(ordered.Encode(appName, resp.NextKey))
			}
			return page, next, nil
		}

		lister := genericLister[types.AppServer, appServerIndex]{
			engine:          c.engine,
			collection:      c.col,
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

// RangeApplicationServersWithName returns an iterator over application servers for a given app name.
func (c *Cache) RangeApplicationServersWithName(ctx context.Context, appName string) iter.Seq2[types.AppServer, error] {
	return appServerCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		logger:   c.Logger,
		upstream: c.Config.Presence,
		col:      c.collections.appServers,
	}.RangeApplicationServersWithName(ctx, appName)
}
