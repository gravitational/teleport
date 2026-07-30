package crud

import (
	"context"

	"github.com/gravitational/trace"

	dynamicwindowsclient "github.com/gravitational/teleport/api/client/dynamicwindows"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type dynamicWindowsDesktopClientGetter interface {
	DynamicDesktopClient() *dynamicwindowsclient.Client
}

// DynamicWindowsDesktopOpsForClient returns type-erased CRUD operations for dynamic Windows desktop resources.
func DynamicWindowsDesktopOpsForClient(client any) (KindResourceOps, error) {
	if client, ok := client.(services.DynamicWindowsDesktops); ok {
		return DynamicWindowsDesktopOps(client), nil
	}
	if client, ok := client.(dynamicWindowsDesktopClientGetter); ok {
		return DynamicWindowsDesktopOps(client.DynamicDesktopClient()), nil
	}
	return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", types.KindDynamicWindowsDesktop)
}

type dynamicWindowsDesktopOps struct {
	clt services.DynamicWindowsDesktops
}

type dynamicWindowsDesktopKindOps struct {
	*dynamicWindowsDesktopOps
	*resourceOpsLegacy[*dynamicWindowsDesktopOps, types.DynamicWindowsDesktop]
}

// DynamicWindowsDesktopOps returns CRUD operations for dynamic Windows desktop resources.
func DynamicWindowsDesktopOps(client services.DynamicWindowsDesktops) KindResourceOps {
	ops := &dynamicWindowsDesktopOps{clt: client}
	return &dynamicWindowsDesktopKindOps{
		dynamicWindowsDesktopOps: ops,
		resourceOpsLegacy:        &resourceOpsLegacy[*dynamicWindowsDesktopOps, types.DynamicWindowsDesktop]{ops: ops},
	}
}

func (o *dynamicWindowsDesktopOps) Kind() string {
	return types.KindDynamicWindowsDesktop
}

func (o *dynamicWindowsDesktopOps) NewResource(name string) (types.Resource153, error) {
	desktop, err := types.NewDynamicWindowsDesktopV1(name, nil, types.DynamicWindowsDesktopSpecV1{
		Addr: "127.0.0.1:3389",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.LegacyToResource153(desktop), nil
}

func (o *dynamicWindowsDesktopOps) Clone(resource types.DynamicWindowsDesktop) types.DynamicWindowsDesktop {
	return resource.Copy()
}

func (o *dynamicWindowsDesktopOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	desktop, err := unwrapLegacy[types.DynamicWindowsDesktop](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	created, err := o.clt.CreateDynamicWindowsDesktop(ctx, desktop)
	return types.LegacyToResource153(created), trace.Wrap(err)
}

func (o *dynamicWindowsDesktopOps) Get(ctx context.Context, name string) (types.DynamicWindowsDesktop, error) {
	return o.clt.GetDynamicWindowsDesktop(ctx, name)
}

func (o *dynamicWindowsDesktopOps) List(ctx context.Context, pageSize int, pageToken string) ([]types.DynamicWindowsDesktop, string, error) {
	desktops, next, err := o.clt.ListDynamicWindowsDesktops(ctx, pageSize, pageToken)
	return desktops, next, trace.Wrap(err)
}

func (o *dynamicWindowsDesktopOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	desktop, err := unwrapLegacy[types.DynamicWindowsDesktop](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updated, err := o.clt.UpdateDynamicWindowsDesktop(ctx, desktop)
	return types.LegacyToResource153(updated), trace.Wrap(err)
}

func (o *dynamicWindowsDesktopOps) Delete(ctx context.Context, name string) error {
	return trace.Wrap(o.clt.DeleteDynamicWindowsDesktop(ctx, name))
}

func (o *dynamicWindowsDesktopOps) DeleteAll(ctx context.Context) error {
	return trace.NotImplemented("delete-all is not implemented for kind %q", o.Kind())
}

func (o *dynamicWindowsDesktopOps) SupportsDeleteAll() bool {
	return false
}

func (o *dynamicWindowsDesktopOps) Equal(a, b types.Resource153) bool {
	return EqualLegacyResource[types.DynamicWindowsDesktop](a, b)
}
