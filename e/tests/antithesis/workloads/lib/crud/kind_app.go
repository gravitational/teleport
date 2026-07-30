package crud

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// AppOpsForClient returns type-erased CRUD operations for app resources.
func AppOpsForClient(client any) (KindResourceOps, error) {
	c, ok := client.(services.Applications)
	if !ok {
		return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", types.KindApp)
	}
	return AppOps(c), nil
}

type appOps struct {
	clt services.Applications
}

type appKindOps struct {
	*appOps
	*resourceOpsLegacy[*appOps, types.Application]
}

// AppOps returns CRUD operations for app resources.
func AppOps(client services.Applications) KindResourceOps {
	ops := &appOps{clt: client}
	return &appKindOps{
		appOps:            ops,
		resourceOpsLegacy: &resourceOpsLegacy[*appOps, types.Application]{ops: ops},
	}
}

func (o *appOps) Kind() string {
	return types.KindApp
}

func (o *appOps) NewResource(name string) (types.Resource153, error) {
	app, err := types.NewAppV3(types.Metadata{Name: name}, types.AppSpecV3{
		URI: "http://localhost:8080",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.LegacyToResource153(app), nil
}

func (o *appOps) Clone(resource types.Application) types.Application {
	return CopyLegacyResourceR[*types.AppV3](resource).(types.Application)
}

func (o *appOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	app, err := unwrapLegacy[types.Application](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, trace.Wrap(o.clt.CreateApp(ctx, app))
}

func (o *appOps) Get(ctx context.Context, name string) (types.Application, error) {
	return o.clt.GetApp(ctx, name)
}

func (o *appOps) List(ctx context.Context, pageSize int, pageToken string) ([]types.Application, string, error) {
	apps, next, err := o.clt.ListApps(ctx, pageSize, pageToken)
	return apps, next, trace.Wrap(err)
}

func (o *appOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	app, err := unwrapLegacy[types.Application](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, trace.Wrap(o.clt.UpdateApp(ctx, app))
}

func (o *appOps) Delete(ctx context.Context, name string) error {
	return trace.Wrap(o.clt.DeleteApp(ctx, name))
}

func (o *appOps) DeleteAll(ctx context.Context) error {
	return trace.Wrap(o.clt.DeleteAllApps(ctx))
}

func (o *appOps) SupportsDeleteAll() bool {
	return true
}

func (o *appOps) Equal(a, b types.Resource153) bool {
	return EqualLegacyResource[types.Application](a, b)
}
