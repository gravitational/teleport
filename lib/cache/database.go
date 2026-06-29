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
	"google.golang.org/protobuf/proto"
	"rsc.io/ordered"

	"github.com/gravitational/teleport/api/client"
	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

type databaseIndex string

const databaseNameIndex = "name"

func newDatabaseCollection(upstream services.Databases, w types.WatchKind) (*collection[types.Database, databaseIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Databases")
	}

	return &collection[types.Database, databaseIndex]{
		store: newStore(
			types.KindDatabase,
			func(d types.Database) types.Database {
				return d.Copy()
			},
			map[databaseIndex]func(types.Database) string{
				databaseNameIndex: types.Database.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Database, error) {
			// TODO(lokraszewski): DELETE IN v21.0.0 replace by regular clientutils.Resources
			out, err := clientutils.CollectWithFallback(ctx, upstream.ListDatabases, upstream.GetDatabases)
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.Database {
			return &types.DatabaseV3{
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

// GetDatabase returns the specified database resource.
func (c *Cache) GetDatabase(ctx context.Context, name string) (types.Database, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetDatabase")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.dbs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		dbs, err := c.Config.Databases.GetDatabase(ctx, name)
		return dbs, trace.Wrap(err)
	}

	d, err := rg.store.get(databaseNameIndex, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return d.Copy(), nil
}

// GetDatabases returns all database resources.
// Deprecated: Prefer paginated variant such as [ListDatabases] or [RangeDatabases]
func (c *Cache) GetDatabases(ctx context.Context) ([]types.Database, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetDatabases")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.dbs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		dbs, err := c.Config.Databases.GetDatabases(ctx)
		return dbs, trace.Wrap(err)
	}

	out := make([]types.Database, 0, rg.store.len())
	for d := range rg.store.resources(databaseNameIndex, "", "") {
		out = append(out, d.Copy())
	}

	return out, nil
}

// ListDatabases returns a page of database resources.
func (c *Cache) ListDatabases(ctx context.Context, limit int, startKey string) ([]types.Database, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListDatabases")
	defer span.End()

	lister := genericLister[types.Database, databaseIndex]{
		cache:        c,
		collection:   c.collections.dbs,
		index:        databaseNameIndex,
		upstreamList: c.Config.Databases.ListDatabases,
		nextToken:    types.Database.GetName,
	}
	out, next, err := lister.list(ctx, limit, startKey)
	return out, next, trace.Wrap(err)
}

// RangeDatabases returns database resources within the range [start, end).
func (c *Cache) RangeDatabases(ctx context.Context, start, end string) iter.Seq2[types.Database, error] {
	lister := genericLister[types.Database, databaseIndex]{
		cache:        c,
		collection:   c.collections.dbs,
		index:        databaseNameIndex,
		upstreamList: c.Config.Databases.ListDatabases,
		nextToken:    types.Database.GetName,
		// TODO(lokraszewski): DELETE IN v21.0.0
		fallbackGetter: c.Config.Databases.GetDatabases,
	}

	return func(yield func(types.Database, error) bool) {
		ctx, span := c.Tracer.Start(ctx, "cache/RangeDatabases")
		defer span.End()

		for db, err := range lister.RangeWithFallback(ctx, start, end) {
			if !yield(db, err) {
				return
			}

			if err != nil {
				return
			}
		}
	}
}

type databaseServerIndex string

const databaseServerNameIndex databaseServerIndex = "name"
const databaseServerDatabaseNameIndex databaseServerIndex = "database_name"

func databaseServerByDatabaseNameKey(s types.DatabaseServer) string {
	// Delete events deliver header only resources with a nil Database. This
	// returns "" so the secondary index lookup is a no-op. The primary
	// index deletion removes the entry from all indexes.
	db := s.GetDatabase()
	if db == nil {
		return ""
	}
	return string(ordered.Encode(db.GetName(), s.GetHostID(), s.GetName()))
}

func newDatabaseServerCollection(p services.Presence, w types.WatchKind) (*collection[types.DatabaseServer, databaseServerIndex], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[types.DatabaseServer, databaseServerIndex]{
		store: newStore(
			types.KindDatabaseServer,
			types.DatabaseServer.Copy,
			map[databaseServerIndex]func(types.DatabaseServer) string{
				databaseServerNameIndex: func(u types.DatabaseServer) string {
					return u.GetHostID() + "/" + u.GetName()
				},
				databaseServerDatabaseNameIndex: databaseServerByDatabaseNameKey,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.DatabaseServer, error) {
			return p.GetDatabaseServers(ctx, defaults.Namespace)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.DatabaseServer {
			return &types.DatabaseServerV3{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
				Spec: types.DatabaseServerSpecV3{
					HostID: hdr.Metadata.Description,
				},
			}
		},
		watch: w,
	}, nil
}

// GetDatabaseServers returns all registered database proxy servers.
func (c *Cache) GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetDatabaseServers")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.dbServers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		servers, err := c.Config.Presence.GetDatabaseServers(ctx, namespace)
		return servers, trace.Wrap(err)
	}

	out := make([]types.DatabaseServer, 0, rg.store.len())
	for ds := range rg.store.resources(databaseServerNameIndex, "", "") {
		out = append(out, ds.Copy())
	}

	return out, nil
}

// RangeDatabaseServersWithName returns an iterator over database proxy servers for a given database name.
func (c *Cache) RangeDatabaseServersWithName(ctx context.Context, databaseName string) iter.Seq2[types.DatabaseServer, error] {
	if databaseName == "" {
		return stream.Fail[types.DatabaseServer](trace.BadParameter("missing database name"))
	}

	return func(yield func(types.DatabaseServer, error) bool) {
		ctx, span := c.Tracer.Start(ctx, "cache/RangeDatabaseServersWithName")
		defer span.End()

		upstreamListFn := func(ctx context.Context, pageSize int, startToken string) ([]types.DatabaseServer, string, error) {
			var tokenDatabaseName string
			rest, err := ordered.DecodePrefix([]byte(startToken), &tokenDatabaseName)
			if err != nil {
				return nil, "", trace.Wrap(err)
			}

			// Verify that the token's database name matches the requested database name.
			// This ensures that if the token is malformed or belongs to a different
			// database, we don't return incorrect results.
			if tokenDatabaseName != databaseName {
				return nil, "", trace.BadParameter("pagination token does not match the requested database name")
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
				ResourceType: types.KindDatabaseServer,
				Namespace:    defaults.Namespace,
				Limit:        int32(pageSize),
				StartKey:     backendKey,
			})
			if err != nil {
				return nil, "", trace.Wrap(err)
			}

			var page []types.DatabaseServer
			for _, r := range resp.Resources {
				server, ok := r.(types.DatabaseServer)
				if !ok {
					c.Logger.WarnContext(ctx, "expected DatabaseServer but received unexpected type", "resource_type", logutils.TypeAttr(r))
					continue
				}
				if server.GetDatabase().GetName() == databaseName {
					page = append(page, server)
				}
			}

			next := ""
			if resp.NextKey != "" {
				hostID, serverName, ok := strings.Cut(resp.NextKey, backend.SeparatorString)
				if !ok {
					return nil, "", trace.BadParameter("invalid pagination token: %q", resp.NextKey)
				}
				next = string(ordered.Encode(databaseName, hostID, serverName))
			}
			return page, next, nil
		}

		lister := genericLister[types.DatabaseServer, databaseServerIndex]{
			cache:           c,
			collection:      c.collections.dbServers,
			index:           databaseServerDatabaseNameIndex,
			nextToken:       databaseServerByDatabaseNameKey,
			defaultPageSize: defaults.DefaultChunkSize,
			upstreamList:    upstreamListFn,
		}

		start := string(ordered.Encode(databaseName))
		end := string(ordered.Encode(databaseName, ordered.Inf))
		for item, err := range lister.Range(ctx, start, end) {
			if !yield(item, err) {
				return
			}
		}
	}
}

type databaseServiceIndex string

const databaseServiceNameIndex databaseServiceIndex = "name"

func newDatabaseServiceCollection(p services.Presence, w types.WatchKind) (*collection[types.DatabaseService, databaseServiceIndex], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Databases")
	}

	return &collection[types.DatabaseService, databaseServiceIndex]{
		store: newStore(
			types.KindDatabaseService,
			types.DatabaseService.Clone,
			map[databaseServiceIndex]func(types.DatabaseService) string{
				databaseServiceNameIndex: types.DatabaseService.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.DatabaseService, error) {
			resources, err := client.GetResourcesWithFilters(ctx, p, clientproto.ListResourcesRequest{ResourceType: types.KindDatabaseService})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			dbsvcs := make([]types.DatabaseService, 0, len(resources))
			for _, resource := range resources {
				dbsvc, ok := resource.(types.DatabaseService)
				if !ok {
					return nil, trace.BadParameter("unexpected resource %T", resource)
				}
				dbsvcs = append(dbsvcs, dbsvc)
			}

			return dbsvcs, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.DatabaseService {
			return &types.DatabaseServiceV1{
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

type databaseObjectIndex string

const databaseObjectNameIndex databaseObjectIndex = "name"

func newDatabaseObjectCollection(upstream services.DatabaseObjects, w types.WatchKind) (*collection[*dbobjectv1.DatabaseObject, databaseObjectIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter DatabaseObjects")
	}

	return &collection[*dbobjectv1.DatabaseObject, databaseObjectIndex]{
		store: newStore(
			types.KindDatabaseObject,
			proto.CloneOf[*dbobjectv1.DatabaseObject],
			map[databaseObjectIndex]func(*dbobjectv1.DatabaseObject) string{
				databaseObjectNameIndex: func(r *dbobjectv1.DatabaseObject) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*dbobjectv1.DatabaseObject, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListDatabaseObjects))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *dbobjectv1.DatabaseObject {
			return dbobjectv1.DatabaseObject_builder{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: headerv1.Metadata_builder{
					Name: hdr.Metadata.Name,
				}.Build(),
			}.Build()
		},
		watch: w,
	}, nil
}

func (c *Cache) GetDatabaseObject(ctx context.Context, name string) (*dbobjectv1.DatabaseObject, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetDatabaseObject")
	defer span.End()

	getter := genericGetter[*dbobjectv1.DatabaseObject, databaseObjectIndex]{
		cache:       c,
		collection:  c.collections.databaseObjects,
		index:       databaseObjectNameIndex,
		upstreamGet: c.Config.DatabaseObjects.GetDatabaseObject,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

func (c *Cache) ListDatabaseObjects(ctx context.Context, size int, pageToken string) ([]*dbobjectv1.DatabaseObject, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListDatabaseObjects")
	defer span.End()

	lister := genericLister[*dbobjectv1.DatabaseObject, databaseObjectIndex]{
		cache:        c,
		collection:   c.collections.databaseObjects,
		index:        databaseObjectNameIndex,
		upstreamList: c.Config.DatabaseObjects.ListDatabaseObjects,
		nextToken: func(dbo *dbobjectv1.DatabaseObject) string {
			return dbo.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, size, pageToken)
	return out, next, trace.Wrap(err)
}
