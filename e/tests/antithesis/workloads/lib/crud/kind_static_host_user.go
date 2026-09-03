package crud

import (
	"context"

	"github.com/gravitational/trace"

	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	userprovisioningv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userprovisioning"
	"github.com/gravitational/teleport/lib/services"
)

type staticHostUserClientGetter interface {
	StaticHostUserClient() services.StaticHostUser
}

// StaticHostUserOpsForClient returns type-erased CRUD operations for static host user resources.
func StaticHostUserOpsForClient(client any) (KindResourceOps, error) {
	if client, ok := client.(services.StaticHostUser); ok {
		return StaticHostUserOps(client), nil
	}
	if client, ok := client.(staticHostUserClientGetter); ok {
		return StaticHostUserOps(client.StaticHostUserClient()), nil
	}
	return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", types.KindStaticHostUser)
}

type staticHostUserOps struct {
	clt services.StaticHostUser
}

type staticHostUserKindOps struct {
	*staticHostUserOps
	*resourceOpsProto153[*userprovisioningv2.StaticHostUser, userprovisioningv2.StaticHostUser, *staticHostUserOps]
}

// StaticHostUserOps returns CRUD operations for static host user resources.
func StaticHostUserOps(client services.StaticHostUser) KindResourceOps {
	ops := &staticHostUserOps{clt: client}
	return &staticHostUserKindOps{
		staticHostUserOps:   ops,
		resourceOpsProto153: &resourceOpsProto153[*userprovisioningv2.StaticHostUser, userprovisioningv2.StaticHostUser, *staticHostUserOps]{ops: ops},
	}
}

func (o *staticHostUserOps) Kind() string {
	return types.KindStaticHostUser
}

func (o *staticHostUserOps) NewResource(name string) (types.Resource153, error) {
	return userprovisioning.NewStaticHostUser(name, userprovisioningv2.StaticHostUserSpec_builder{
		Matchers: []*userprovisioningv2.Matcher{
			userprovisioningv2.Matcher_builder{
				NodeLabels: []*labelv1.Label{
					labelv1.Label_builder{Name: "crud", Values: []string{"true"}}.Build(),
				},
			}.Build(),
		},
	}.Build()), nil
}

func (o *staticHostUserOps) Clone(resource *userprovisioningv2.StaticHostUser) *userprovisioningv2.StaticHostUser {
	return CloneProto(resource)
}

func (o *staticHostUserOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	created, err := o.clt.CreateStaticHostUser(ctx, mustCastResource[*userprovisioningv2.StaticHostUser](resource))
	return created, trace.Wrap(err)
}

func (o *staticHostUserOps) Get(ctx context.Context, name string) (*userprovisioningv2.StaticHostUser, error) {
	return o.clt.GetStaticHostUser(ctx, name)
}

func (o *staticHostUserOps) List(ctx context.Context, pageSize int, pageToken string) ([]*userprovisioningv2.StaticHostUser, string, error) {
	users, next, err := o.clt.ListStaticHostUsers(ctx, pageSize, pageToken)
	return users, next, trace.Wrap(err)
}

func (o *staticHostUserOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	updated, err := o.clt.UpdateStaticHostUser(ctx, mustCastResource[*userprovisioningv2.StaticHostUser](resource))
	return updated, trace.Wrap(err)
}

func (o *staticHostUserOps) Delete(ctx context.Context, name string) error {
	return trace.Wrap(o.clt.DeleteStaticHostUser(ctx, name))
}

func (o *staticHostUserOps) DeleteAll(ctx context.Context) error {
	return trace.NotImplemented("delete-all is not implemented for kind %q", o.Kind())
}

func (o *staticHostUserOps) SupportsDeleteAll() bool {
	return false
}

func (o *staticHostUserOps) Equal(a, b types.Resource153) bool {
	return EqualProtoMessage(
		mustCastResource[*userprovisioningv2.StaticHostUser](a),
		mustCastResource[*userprovisioningv2.StaticHostUser](b),
	)
}
