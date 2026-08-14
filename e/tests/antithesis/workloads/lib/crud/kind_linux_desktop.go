package crud

import (
	"context"

	"github.com/gravitational/trace"

	linuxdesktopclient "github.com/gravitational/teleport/api/client/linuxdesktop"
	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
	linuxdesktop "github.com/gravitational/teleport/lib/auth/linuxdesktop/linuxdesktopv1"
	"github.com/gravitational/teleport/lib/services"
)

type linuxDesktopClientGetter interface {
	LinuxDesktopClient() *linuxdesktopclient.Client
}

// LinuxDesktopOpsForClient returns type-erased CRUD operations for Linux desktop resources.
func LinuxDesktopOpsForClient(client any) (KindResourceOps, error) {
	if client, ok := client.(services.LinuxDesktops); ok {
		return LinuxDesktopOps(client), nil
	}
	if client, ok := client.(linuxDesktopClientGetter); ok {
		return LinuxDesktopOps(client.LinuxDesktopClient()), nil
	}
	return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", types.KindLinuxDesktop)
}

type linuxDesktopOps struct {
	clt services.LinuxDesktops
}

type linuxDesktopKindOps struct {
	*linuxDesktopOps
	*resourceOpsProto153[*linuxdesktopv1.LinuxDesktop, linuxdesktopv1.LinuxDesktop, *linuxDesktopOps]
}

// LinuxDesktopOps returns CRUD operations for Linux desktop resources.
func LinuxDesktopOps(client services.LinuxDesktops) KindResourceOps {
	ops := &linuxDesktopOps{clt: client}
	return &linuxDesktopKindOps{
		linuxDesktopOps:     ops,
		resourceOpsProto153: &resourceOpsProto153[*linuxdesktopv1.LinuxDesktop, linuxdesktopv1.LinuxDesktop, *linuxDesktopOps]{ops: ops},
	}
}

func (o *linuxDesktopOps) Kind() string {
	return types.KindLinuxDesktop
}

func (o *linuxDesktopOps) NewResource(name string) (types.Resource153, error) {
	return linuxdesktop.NewLinuxDesktop(name, &linuxdesktopv1.LinuxDesktopSpec{
		Addr:     "127.0.0.1:3022",
		Hostname: name,
	})
}

func (o *linuxDesktopOps) Clone(resource *linuxdesktopv1.LinuxDesktop) *linuxdesktopv1.LinuxDesktop {
	return CloneProto(resource)
}

func (o *linuxDesktopOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	created, err := o.clt.CreateLinuxDesktop(ctx, mustCastResource[*linuxdesktopv1.LinuxDesktop](resource))
	return created, trace.Wrap(err)
}

func (o *linuxDesktopOps) Get(ctx context.Context, name string) (*linuxdesktopv1.LinuxDesktop, error) {
	return o.clt.GetLinuxDesktop(ctx, name)
}

func (o *linuxDesktopOps) List(ctx context.Context, pageSize int, pageToken string) ([]*linuxdesktopv1.LinuxDesktop, string, error) {
	desktops, next, err := o.clt.ListLinuxDesktops(ctx, pageSize, pageToken)
	return desktops, next, trace.Wrap(err)
}

func (o *linuxDesktopOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	updated, err := o.clt.UpdateLinuxDesktop(ctx, mustCastResource[*linuxdesktopv1.LinuxDesktop](resource))
	return updated, trace.Wrap(err)
}

func (o *linuxDesktopOps) Delete(ctx context.Context, name string) error {
	return trace.Wrap(o.clt.DeleteLinuxDesktop(ctx, name))
}

func (o *linuxDesktopOps) DeleteAll(ctx context.Context) error {
	return trace.NotImplemented("delete-all is not implemented for kind %q", o.Kind())
}

func (o *linuxDesktopOps) SupportsDeleteAll() bool {
	return false
}

func (o *linuxDesktopOps) Equal(a, b types.Resource153) bool {
	return EqualProtoMessage(
		mustCastResource[*linuxdesktopv1.LinuxDesktop](a),
		mustCastResource[*linuxdesktopv1.LinuxDesktop](b),
	)
}
