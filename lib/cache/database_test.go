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
	"cmp"
	"context"
	"slices"
	"strconv"
	"testing"
	"time"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/itertools/stream"
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
		list: func(ctx context.Context) ([]types.DatabaseService, error) {
			resources, err := listAllResource(t, p.presenceS, types.KindDatabaseService, apidefaults.DefaultChunkSize)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return types.ResourcesWithLabels(resources).AsDatabaseServices()
		},
		cacheList: func(ctx context.Context, pageSize int) ([]types.DatabaseService, error) {
			resources, err := listAllResource(t, p.cache, types.KindDatabaseService, pageSize)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return types.ResourcesWithLabels(resources).AsDatabaseServices()
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
		create: p.databases.CreateDatabase,
		list: func(ctx context.Context) ([]types.Database, error) {
			return stream.Collect(p.databases.RangeDatabases(ctx, "", ""))
		},
		cacheGet: p.cache.GetDatabase,
		cacheList: func(ctx context.Context, _ int) ([]types.Database, error) {
			return stream.Collect(p.cache.RangeDatabases(ctx, "", ""))
		},
		update:    p.databases.UpdateDatabase,
		deleteAll: p.databases.DeleteAllDatabases,
	})
}

func TestDatabasesPagination(t *testing.T) {
	// TODO(okraport): extract this into generic helper for other paginated resources.
	t.Parallel()

	p, err := newPack(t.TempDir(), ForProxy)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.eventsC: // Drain events to prevent deadlocking.
			}
		}
	}()

	expected := make([]types.Database, 0, 50)
	for i := range 50 {
		db, err := types.NewDatabaseV3(types.Metadata{
			Name: "db" + strconv.Itoa(i+1),
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      "localhost:5432",
		})

		require.NoError(t, err)
		require.NoError(t, p.databases.CreateDatabase(t.Context(), db))
		expected = append(expected, db)
	}
	slices.SortFunc(expected, func(a, b types.Database) int {
		return cmp.Compare(a.GetName(), b.GetName())
	})

	// Wait for all the Databases to be replicated to the cache.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.Equal(t, len(expected), p.cache.collections.dbs.store.len())
	}, 15*time.Second, 100*time.Millisecond)

	out, err := p.cache.GetDatabases(t.Context())
	require.NoError(t, err)
	assert.Len(t, out, len(expected))
	assert.Empty(t, gocmp.Diff(expected, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	page1, page2Start, err := p.cache.ListDatabases(t.Context(), 10, "")
	require.NoError(t, err)
	assert.Len(t, page1, 10)
	assert.NotEmpty(t, page2Start)

	page2, next, err := p.cache.ListDatabases(t.Context(), 1000, page2Start)
	require.NoError(t, err)
	assert.Len(t, page2, len(expected)-10)
	assert.Empty(t, next)

	listed := append(page1, page2...)
	assert.Empty(t, gocmp.Diff(expected, listed,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	out, err = stream.Collect(p.cache.RangeDatabases(t.Context(), "", page2Start))
	require.NoError(t, err)
	assert.Len(t, out, len(page1))
	assert.Empty(t, gocmp.Diff(page1, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	out, err = stream.Collect(p.cache.RangeDatabases(t.Context(), "", ""))
	require.NoError(t, err)
	assert.Len(t, out, len(expected))
	assert.Empty(t, gocmp.Diff(expected, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	out, err = stream.Collect(p.cache.RangeDatabases(t.Context(), page2Start, ""))
	require.NoError(t, err)
	assert.Len(t, out, len(expected)-10)
	assert.Empty(t, gocmp.Diff(expected, append(page1, out...),
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))

	// invalidate the cache, cover upstream fallback
	p.cache.ok = false
	out, err = stream.Collect(p.cache.RangeDatabases(t.Context(), "", ""))
	require.NoError(t, err)
	assert.Len(t, out, len(expected))
	assert.Empty(t, gocmp.Diff(expected, out,
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
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
			list: func(ctx context.Context) ([]types.DatabaseServer, error) {
				return p.presenceS.GetDatabaseServers(ctx, apidefaults.Namespace)
			},
			cacheList: func(ctx context.Context, pageSize int) ([]types.DatabaseServer, error) {
				return p.cache.GetDatabaseServers(ctx, apidefaults.Namespace)
			},
			update: withKeepalive(p.presenceS.UpsertDatabaseServer),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllDatabaseServers(ctx, apidefaults.Namespace)
			},
		})
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
			list: func(ctx context.Context) ([]types.DatabaseServer, error) {
				resources, err := listAllResource(t, p.presenceS, types.KindDatabaseServer, apidefaults.DefaultChunkSize)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				return types.ResourcesWithLabels(resources).AsDatabaseServers()
			},
			cacheList: func(ctx context.Context, pageSize int) ([]types.DatabaseServer, error) {
				resources, err := listAllResource(t, p.cache, types.KindDatabaseServer, pageSize)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				return types.ResourcesWithLabels(resources).AsDatabaseServers()
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

	testResources153(t, p, testFuncs153[*dbobjectv1.DatabaseObject]{
		newResource: func(name string) (*dbobjectv1.DatabaseObject, error) {
			return newDatabaseObject(t, name), nil
		},
		create: func(ctx context.Context, item *dbobjectv1.DatabaseObject) error {
			_, err := p.databaseObjects.CreateDatabaseObject(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*dbobjectv1.DatabaseObject, error) {
			items, _, err := p.databaseObjects.ListDatabaseObjects(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*dbobjectv1.DatabaseObject, error) {
			items, _, err := p.databaseObjects.ListDatabaseObjects(ctx, 0, "")
			return items, trace.Wrap(err)
		},
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
