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
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

// TestDatabaseServices tests that CRUD operations on DatabaseServices are
// replicated from the backend to the cache.
func TestDatabaseServices(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.DatabaseService]{
		newResource: func(name string) (types.DatabaseService, error) {
			return types.NewDatabaseServiceV1(types.Metadata{
				Name: name,
			}, types.DatabaseServiceSpecV1{
				ResourceMatchers: []*types.DatabaseResourceMatcher{
					{Labels: &types.Labels{"env": []string{"prod"}}},
				},
			})
		},
		create: withKeepalive(p.databaseServices.UpsertDatabaseService),
		list: func(ctx context.Context, pageSize int, pageToken string) ([]types.DatabaseService, string, error) {
			resources, next, err := listResource(ctx, p.presenceS, types.KindDatabaseService, pageSize, pageToken)
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			dbs, err := types.ResourcesWithLabels(resources).AsDatabaseServices()
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			return dbs, next, nil
		},
		cacheList: func(ctx context.Context, pageSize int, pageToken string) ([]types.DatabaseService, string, error) {
			resources, next, err := listResource(ctx, p.cache, types.KindDatabaseService, pageSize, pageToken)
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			dbs, err := types.ResourcesWithLabels(resources).AsDatabaseServices()
			if err != nil {
				return nil, "", trace.Wrap(err)
			}
			return dbs, next, nil
		},
		update:    withKeepalive(p.databaseServices.UpsertDatabaseService),
		deleteAll: p.databaseServices.DeleteAllDatabaseServices,
	})
}

// TestDatabases tests that CRUD operations on database resources are
// replicated from the backend to the cache.
func TestDatabases(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.Database]{
		newResource: func(name string) (types.Database, error) {
			return types.NewDatabaseV3(types.Metadata{
				Name: name,
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
			})
		},
		create:     p.databases.CreateDatabase,
		list:       p.databases.ListDatabases,
		Range:      p.databases.RangeDatabases,
		cacheGet:   p.cache.GetDatabase,
		cacheList:  p.cache.ListDatabases,
		cacheRange: p.cache.RangeDatabases,
		update:     p.databases.UpdateDatabase,
		deleteAll:  p.databases.DeleteAllDatabases,
	})
}

// TestDatabaseServers tests that CRUD operations on database servers are
// replicated from the backend to the cache.
func TestDatabaseServers(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	t.Run("GetDatabaseServers", func(t *testing.T) {
		testResources(t, p, testFuncs[types.DatabaseServer]{
			newResource: func(name string) (types.DatabaseServer, error) {
				return types.NewDatabaseServerV3(types.Metadata{
					Name: name,
				}, types.DatabaseServerSpecV3{
					Database: mustCreateDatabase(t, name, defaults.ProtocolPostgres, "localhost:5432"),
					Hostname: "localhost",
					HostID:   uuid.New().String(),
				})
			},
			create: withKeepalive(p.presenceS.UpsertDatabaseServer),
			list: getAllAdapter(func(ctx context.Context) ([]types.DatabaseServer, error) {
				return p.presenceS.GetDatabaseServers(ctx, apidefaults.Namespace)
			}),
			cacheList: getAllAdapter(func(ctx context.Context) ([]types.DatabaseServer, error) {
				return p.cache.GetDatabaseServers(ctx, apidefaults.Namespace)
			}),
			update: withKeepalive(p.presenceS.UpsertDatabaseServer),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllDatabaseServers(ctx, apidefaults.Namespace)
			},
		}, withSkipPaginationTest())
	})

	t.Run("ListResources", func(t *testing.T) {
		testResources(t, p, testFuncs[types.DatabaseServer]{
			newResource: func(name string) (types.DatabaseServer, error) {
				return types.NewDatabaseServerV3(types.Metadata{
					Name: name,
				}, types.DatabaseServerSpecV3{
					Database: mustCreateDatabase(t, name, defaults.ProtocolPostgres, "localhost:5432"),
					Hostname: "localhost",
					HostID:   uuid.New().String(),
				})
			},
			create: withKeepalive(p.presenceS.UpsertDatabaseServer),
			list: func(ctx context.Context, pageSize int, pageToken string) ([]types.DatabaseServer, string, error) {
				resources, next, err := listResource(ctx, p.presenceS, types.KindDatabaseServer, pageSize, pageToken)
				if err != nil {
					return nil, "", trace.Wrap(err)
				}
				dbs, err := types.ResourcesWithLabels(resources).AsDatabaseServers()
				if err != nil {
					return nil, "", trace.Wrap(err)
				}
				return dbs, next, nil
			},
			cacheList: func(ctx context.Context, pageSize int, pageToken string) ([]types.DatabaseServer, string, error) {
				resources, next, err := listResource(ctx, p.cache, types.KindDatabaseServer, pageSize, pageToken)
				if err != nil {
					return nil, "", trace.Wrap(err)
				}
				dbs, err := types.ResourcesWithLabels(resources).AsDatabaseServers()
				if err != nil {
					return nil, "", trace.Wrap(err)
				}
				return dbs, next, nil
			},
			update: withKeepalive(p.presenceS.UpsertDatabaseServer),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllDatabaseServers(ctx, apidefaults.Namespace)
			},
		})
	})
}

func TestDatabaseObjects(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs[*dbobjectv1.DatabaseObject]{
		newResource: func(name string) (*dbobjectv1.DatabaseObject, error) {
			return newDatabaseObject(t, name), nil
		},
		create: func(ctx context.Context, item *dbobjectv1.DatabaseObject) error {
			_, err := p.databaseObjects.CreateDatabaseObject(ctx, item)
			return trace.Wrap(err)
		},
		list:      p.databaseObjects.ListDatabaseObjects,
		cacheList: p.databaseObjects.ListDatabaseObjects,
		deleteAll: func(ctx context.Context) error {
			token := ""
			var objects []*dbobjectv1.DatabaseObject

			for {
				resp, nextToken, err := p.databaseObjects.ListDatabaseObjects(ctx, 0, token)
				if err != nil {
					return err
				}

				objects = append(objects, resp...)

				if nextToken == "" {
					break
				}
				token = nextToken
			}

			for _, object := range objects {
				err := p.databaseObjects.DeleteDatabaseObject(ctx, object.GetMetadata().GetName())
				if err != nil {
					return err
				}
			}
			return nil
		},
	})
}
