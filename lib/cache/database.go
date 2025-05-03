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

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type databaseIndex string

const databaseNameIndex = "name"

func newDatabaseCollection(p services.Databases, w types.WatchKind) (*collection[types.Database, databaseIndex], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Databases")
	}

	return &collection[types.Database, databaseIndex]{
		store: newStore(map[databaseIndex]func(types.Database) string{
			databaseNameIndex: func(u types.Database) string {
				return u.GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Database, error) {
			return p.GetDatabases(ctx)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.Database {
			return &types.DatabaseV3{
				Kind:    types.KindDatabase,
				Version: types.V3,
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

type databaseServerIndex string

const databaseServerNameIndex databaseServerIndex = "name"

func newDatabaseServerCollection(p services.Presence, w types.WatchKind) (*collection[types.DatabaseServer, databaseServerIndex], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[types.DatabaseServer, databaseServerIndex]{
		store: newStore(map[databaseServerIndex]func(types.DatabaseServer) string{
			databaseServerNameIndex: func(u types.DatabaseServer) string {
				return u.GetHostID() + "/" + u.GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.DatabaseServer, error) {
			return p.GetDatabaseServers(ctx, defaults.Namespace)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.DatabaseServer {
			return &types.DatabaseServerV3{
				Kind:    types.KindDatabaseServer,
				Version: types.V3,
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

type databaseServiceIndex string

const databaseServiceNameIndex databaseServiceIndex = "name"

func newDatabaseServiceCollection(p services.Presence, w types.WatchKind) (*collection[types.DatabaseService, databaseServiceIndex], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Databases")
	}

	return &collection[types.DatabaseService, databaseServiceIndex]{
		store: newStore(map[databaseServiceIndex]func(types.DatabaseService) string{
			databaseServiceNameIndex: func(u types.DatabaseService) string {
				return u.GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.DatabaseService, error) {
			resources, err := client.GetResourcesWithFilters(ctx, p, proto.ListResourcesRequest{ResourceType: types.KindDatabaseService})
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
					Kind:    types.KindDatabase,
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
