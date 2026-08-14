package crud

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// DatabaseOpsForClient returns type-erased CRUD operations for database resources.
func DatabaseOpsForClient(client any) (KindResourceOps, error) {
	c, ok := client.(services.Databases)
	if !ok {
		return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", types.KindDatabase)
	}
	return DatabaseOps(c), nil
}

type databaseOps struct {
	clt services.Databases
}

type databaseKindOps struct {
	*databaseOps
	*resourceOpsLegacy[*databaseOps, types.Database]
}

// DatabaseOps returns CRUD operations for database resources.
func DatabaseOps(client services.Databases) KindResourceOps {
	ops := &databaseOps{clt: client}
	return &databaseKindOps{
		databaseOps:       ops,
		resourceOpsLegacy: &resourceOpsLegacy[*databaseOps, types.Database]{ops: ops},
	}
}

func (o *databaseOps) Kind() string {
	return types.KindDatabase
}

func (o *databaseOps) NewResource(name string) (types.Resource153, error) {
	database, err := types.NewDatabaseV3(types.Metadata{Name: name}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.LegacyToResource153(database), nil
}

func (o *databaseOps) Clone(resource types.Database) types.Database {
	return CopyLegacyResourceR[*types.DatabaseV3](resource).(types.Database)
}

func (o *databaseOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	database, err := unwrapLegacy[types.Database](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, trace.Wrap(o.clt.CreateDatabase(ctx, database))
}

func (o *databaseOps) Get(ctx context.Context, name string) (types.Database, error) {
	return o.clt.GetDatabase(ctx, name)
}

func (o *databaseOps) List(ctx context.Context, pageSize int, pageToken string) ([]types.Database, string, error) {
	databases, next, err := o.clt.ListDatabases(ctx, pageSize, pageToken)
	return databases, next, trace.Wrap(err)
}

func (o *databaseOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	database, err := unwrapLegacy[types.Database](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, trace.Wrap(o.clt.UpdateDatabase(ctx, database))
}

func (o *databaseOps) Delete(ctx context.Context, name string) error {
	return trace.Wrap(o.clt.DeleteDatabase(ctx, name))
}

func (o *databaseOps) DeleteAll(ctx context.Context) error {
	return trace.Wrap(o.clt.DeleteAllDatabases(ctx))
}

func (o *databaseOps) SupportsDeleteAll() bool {
	return true
}

func (o *databaseOps) Equal(a, b types.Resource153) bool {
	return EqualLegacyResource[types.Database](a, b)
}
