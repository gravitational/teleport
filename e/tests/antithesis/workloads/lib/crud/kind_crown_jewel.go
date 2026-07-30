package crud

import (
	"context"

	"github.com/gravitational/trace"

	crownjewelclient "github.com/gravitational/teleport/api/client/crownjewel"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/crownjewel"
	"github.com/gravitational/teleport/lib/services"
)

type crownJewelClientGetter interface {
	CrownJewelServiceClient() *crownjewelclient.Client
}

// CrownJewelOpsForClient returns type-erased CRUD operations for crown jewel resources.
func CrownJewelOpsForClient(client any) (KindResourceOps, error) {
	if client, ok := client.(services.CrownJewels); ok {
		return CrownJewelOps(client), nil
	}
	if client, ok := client.(crownJewelClientGetter); ok {
		return CrownJewelOps(client.CrownJewelServiceClient()), nil
	}
	return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", types.KindCrownJewel)
}

type crownJewelOps struct {
	clt services.CrownJewels
}

type crownJewelKindOps struct {
	*crownJewelOps
	*resourceOpsProto153[*crownjewelv1.CrownJewel, crownjewelv1.CrownJewel, *crownJewelOps]
}

// CrownJewelOps returns CRUD operations for crown jewel resources.
func CrownJewelOps(client services.CrownJewels) KindResourceOps {
	ops := &crownJewelOps{clt: client}
	return &crownJewelKindOps{
		crownJewelOps:       ops,
		resourceOpsProto153: &resourceOpsProto153[*crownjewelv1.CrownJewel, crownjewelv1.CrownJewel, *crownJewelOps]{ops: ops},
	}
}

func (o *crownJewelOps) Kind() string {
	return types.KindCrownJewel
}

func (o *crownJewelOps) NewResource(name string) (types.Resource153, error) {
	return crownjewel.NewCrownJewel(name, &crownjewelv1.CrownJewelSpec{
		Query: "SELECT * FROM nodes",
	})
}

func (o *crownJewelOps) Clone(resource *crownjewelv1.CrownJewel) *crownjewelv1.CrownJewel {
	return CloneProto(resource)
}

func (o *crownJewelOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	created, err := o.clt.CreateCrownJewel(ctx, mustCastResource[*crownjewelv1.CrownJewel](resource))
	return created, trace.Wrap(err)
}

func (o *crownJewelOps) Get(ctx context.Context, name string) (*crownjewelv1.CrownJewel, error) {
	return o.clt.GetCrownJewel(ctx, name)
}

func (o *crownJewelOps) List(ctx context.Context, pageSize int, pageToken string) ([]*crownjewelv1.CrownJewel, string, error) {
	resources, next, err := o.clt.ListCrownJewels(ctx, int64(pageSize), pageToken)
	return resources, next, trace.Wrap(err)
}

func (o *crownJewelOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	updated, err := o.clt.UpdateCrownJewel(ctx, mustCastResource[*crownjewelv1.CrownJewel](resource))
	return updated, trace.Wrap(err)
}

func (o *crownJewelOps) Delete(ctx context.Context, name string) error {
	return trace.Wrap(o.clt.DeleteCrownJewel(ctx, name))
}

func (o *crownJewelOps) DeleteAll(ctx context.Context) error {
	return trace.NotImplemented("delete-all is not implemented for kind %q", o.Kind())
}

func (o *crownJewelOps) SupportsDeleteAll() bool {
	return false
}

func (o *crownJewelOps) Equal(a, b types.Resource153) bool {
	return EqualProtoMessage(
		mustCastResource[*crownjewelv1.CrownJewel](a),
		mustCastResource[*crownjewelv1.CrownJewel](b),
	)
}
