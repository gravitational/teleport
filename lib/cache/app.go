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
	"rsc.io/ordered"

	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
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
				appNameIndex: services.GetCursorForApp,
			}),
		fetcher: func(ctx context.Context, _ bool) ([]types.Application, error) {
			out, err := stream.Collect(upstream.RangeApplications(ctx, presencev1.ListApplicationsRequest_builder{
				ScopeFilter: w.ScopeFilter.ToProto(),
			}.Build()))
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

// ListApplications returns a page of registered applications respecting scope filters.
func (c *Cache) ListApplications(ctx context.Context, req *presencev1.ListApplicationsRequest) ([]types.Application, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListApplications")
	defer span.End()

	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, "", trace.Wrap(err)
	}

	apps, next, err := c.appLister(scopeFilter).list(ctx, int(req.GetPageSize()), req.GetPageToken())
	return apps, next, trace.Wrap(err)
}

// RangeApplications returns applications within the range [start, end).
func (c *Cache) RangeApplications(ctx context.Context, req *presencev1.ListApplicationsRequest) iter.Seq2[types.Application, error] {
	ctx, span := c.Tracer.Start(ctx, "cache/RangeApplications")
	defer span.End()

	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return stream.Fail[types.Application](trace.Wrap(err))
	}

	return c.appLister(scopeFilter).Range(ctx, req.GetPageToken(), "")
}

func (c *Cache) appLister(scopeFilter *scopesv1.Filter) genericLister[types.Application, appIndex] {
	return genericLister[types.Application, appIndex]{
		cache:      c,
		collection: c.collections.apps,
		index:      appNameIndex,
		upstreamList: func(ctx context.Context, pageSize int, pageToken string) ([]types.Application, string, error) {
			return c.Config.Apps.ListApplications(ctx, presencev1.ListApplicationsRequest_builder{
				PageSize:    int32(pageSize),
				PageToken:   pageToken,
				ScopeFilter: scopeFilter,
			}.Build())
		},
		nextToken: services.GetCursorForApp,
		filter: func(app types.Application) bool {
			return scopes.MatchScope(scopeFilter, app.GetScope())
		},
	}
}

// GetApplication returns the specified application resource.
func (c *Cache) GetApplication(ctx context.Context, req *presencev1.GetApplicationRequest) (types.Application, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetApplication")
	defer span.End()

	appCursor := scopes.MakeResourceCursor(req.GetScope(), req.GetName())

	getter := genericGetter[types.Application, appIndex]{
		cache:      c,
		collection: c.collections.apps,
		index:      appNameIndex,
		upstreamGet: func(ctx context.Context, _ string) (types.Application, error) {
			return c.Config.Apps.GetApplication(ctx, req)
		},
	}
	out, err := getter.get(ctx, appCursor)
	if out == nil {
		return nil, trace.Wrap(err)
	}
	return out.Copy(), trace.Wrap(err)
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

			// The remainder of the token is the scope aware resource
			// cursor
			startKey := ""
			if len(rest) > 0 {
				if err := ordered.Decode(rest, &startKey); err != nil {
					return nil, "", trace.Wrap(err)
				}
			}

			resp, err := c.Config.Presence.ListResources(ctx, clientproto.ListResourcesRequest{
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
					c.Logger.WarnContext(ctx, "expected AppServer but received unexpected type", "resource_type", logutils.TypeAttr(r))
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
