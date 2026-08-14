package crud

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

const windowsDesktopHostID = "crud-test"

// WindowsDesktopOpsForClient returns type-erased CRUD operations for Windows desktop resources.
func WindowsDesktopOpsForClient(client any) (KindResourceOps, error) {
	c, ok := client.(services.WindowsDesktops)
	if !ok {
		return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", types.KindWindowsDesktop)
	}
	return WindowsDesktopOps(c), nil
}

type windowsDesktopOps struct {
	clt services.WindowsDesktops
}

type windowsDesktopKindOps struct {
	*windowsDesktopOps
	*resourceOpsLegacy[*windowsDesktopOps, types.WindowsDesktop]
}

// WindowsDesktopOps returns CRUD operations for Windows desktop resources.
func WindowsDesktopOps(client services.WindowsDesktops) KindResourceOps {
	ops := &windowsDesktopOps{clt: client}
	return &windowsDesktopKindOps{
		windowsDesktopOps: ops,
		resourceOpsLegacy: &resourceOpsLegacy[*windowsDesktopOps, types.WindowsDesktop]{ops: ops},
	}
}

func (o *windowsDesktopOps) Kind() string {
	return types.KindWindowsDesktop
}

func (o *windowsDesktopOps) NewResource(name string) (types.Resource153, error) {
	desktop, err := types.NewWindowsDesktopV3(name, nil, types.WindowsDesktopSpecV3{
		Addr:   "127.0.0.1:3389",
		HostID: windowsDesktopHostID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.LegacyToResource153(desktop), nil
}

func (o *windowsDesktopOps) Clone(resource types.WindowsDesktop) types.WindowsDesktop {
	return resource.Copy()
}

func (o *windowsDesktopOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	desktop, err := unwrapLegacy[types.WindowsDesktop](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, trace.Wrap(o.clt.CreateWindowsDesktop(ctx, desktop))
}

func (o *windowsDesktopOps) Get(ctx context.Context, name string) (types.WindowsDesktop, error) {
	desktops, err := o.clt.GetWindowsDesktops(ctx, types.WindowsDesktopFilter{
		HostID: windowsDesktopHostID,
		Name:   name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(desktops) == 0 {
		return nil, trace.NotFound("windows desktop %q is not found", name)
	}
	return desktops[0], nil
}

func (o *windowsDesktopOps) List(ctx context.Context, pageSize int, pageToken string) ([]types.WindowsDesktop, string, error) {
	rsp, err := o.clt.ListWindowsDesktops(ctx, types.ListWindowsDesktopsRequest{
		WindowsDesktopFilter: types.WindowsDesktopFilter{HostID: windowsDesktopHostID},
		Limit:                pageSize,
		StartKey:             pageToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return rsp.Desktops, rsp.NextKey, nil
}

func (o *windowsDesktopOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	desktop, err := unwrapLegacy[types.WindowsDesktop](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, trace.Wrap(o.clt.UpdateWindowsDesktop(ctx, desktop))
}

func (o *windowsDesktopOps) Delete(ctx context.Context, name string) error {
	return trace.Wrap(o.clt.DeleteWindowsDesktop(ctx, windowsDesktopHostID, name))
}

func (o *windowsDesktopOps) DeleteAll(ctx context.Context) error {
	return trace.Wrap(o.clt.DeleteAllWindowsDesktops(ctx))
}

func (o *windowsDesktopOps) SupportsDeleteAll() bool {
	return true
}

func (o *windowsDesktopOps) Equal(a, b types.Resource153) bool {
	return EqualLegacyResource[types.WindowsDesktop](a, b)
}
