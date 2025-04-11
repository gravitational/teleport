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

func newDatabaseCollection(p services.Databases, w types.WatchKind) (*collection[types.Database], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Databases")
	}

	return &collection[types.Database]{
		store: newStore(map[string]func(types.Database) string{
			"name": func(u types.Database) string {
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

	if rg.ReadCache() {
		d, err := rg.store.get("name", name)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return d.Copy(), nil
	}

	return c.Config.Databases.GetDatabase(ctx, name)

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

	if rg.ReadCache() {
		out := make([]types.Database, 0, rg.store.len())
		for d := range rg.store.resources("name", "", "") {
			out = append(out, d.Copy())
		}

		return out, nil
	}

	dbs, err := c.Config.Databases.GetDatabases(ctx)
	return dbs, trace.Wrap(err)
}

func newDatabaseServerCollection(p services.Presence, w types.WatchKind) (*collection[types.DatabaseServer], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[types.DatabaseServer]{
		store: newStore(map[string]func(types.DatabaseServer) string{
			"name": func(u types.DatabaseServer) string {
				return u.GetName()
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

	if rg.ReadCache() {
		out := make([]types.DatabaseServer, 0, rg.store.len())
		for ds := range rg.store.resources("name", "", "") {
			out = append(out, ds.Copy())
		}

		return out, nil
	}

	servers, err := c.Config.Presence.GetDatabaseServers(ctx, namespace)
	return servers, trace.Wrap(err)
}

func newDatabaseServiceCollection(p services.Presence, w types.WatchKind) (*collection[types.DatabaseService], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Databases")
	}

	return &collection[types.DatabaseService]{
		store: newStore(map[string]func(types.DatabaseService) string{
			"name": func(u types.DatabaseService) string {
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
