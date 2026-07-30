package crud

import (
	"context"

	"github.com/gravitational/trace"

	authpb "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// RoleOpsForClient returns type-erased CRUD operations for role resources.
func RoleOpsForClient(client any) (KindResourceOps, error) {
	c, ok := client.(services.Access)
	if !ok {
		return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", types.KindRole)
	}
	return RoleOps(c), nil
}

type roleOps struct {
	clt services.Access
}

type roleKindOps struct {
	*roleOps
	*resourceOpsLegacy[*roleOps, types.Role]
}

// RoleOps returns CRUD operations for role resources.
func RoleOps(client services.Access) KindResourceOps {
	ops := &roleOps{clt: client}
	return &roleKindOps{
		roleOps:           ops,
		resourceOpsLegacy: &resourceOpsLegacy[*roleOps, types.Role]{ops: ops},
	}
}

func (o *roleOps) Kind() string {
	return types.KindRole
}

func (o *roleOps) NewResource(name string) (types.Resource153, error) {
	role, err := types.NewRole(name, types.RoleSpecV6{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.LegacyToResource153(role), nil
}

func (o *roleOps) Clone(resource types.Role) types.Role {
	return resource.Clone()
}

func (o *roleOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	role, err := unwrapLegacy[types.Role](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	created, err := o.clt.CreateRole(ctx, role)
	return types.LegacyToResource153(created), trace.Wrap(err)
}

func (o *roleOps) Get(ctx context.Context, name string) (types.Role, error) {
	return o.clt.GetRole(ctx, name)
}

func (o *roleOps) List(ctx context.Context, pageSize int, pageToken string) ([]types.Role, string, error) {
	rsp, err := o.clt.ListRoles(ctx, &authpb.ListRolesRequest{
		Limit:    int32(pageSize),
		StartKey: pageToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	roles := make([]types.Role, 0, len(rsp.Roles))
	for _, role := range rsp.Roles {
		roles = append(roles, role)
	}
	return roles, rsp.NextKey, nil
}

func (o *roleOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	role, err := unwrapLegacy[types.Role](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updated, err := o.clt.UpdateRole(ctx, role)
	return types.LegacyToResource153(updated), trace.Wrap(err)
}

func (o *roleOps) Delete(ctx context.Context, name string) error {
	return trace.Wrap(o.clt.DeleteRole(ctx, name))
}

func (o *roleOps) DeleteAll(ctx context.Context) error {
	return trace.NotImplemented("delete-all is not implemented for kind %q", o.Kind())
}

func (o *roleOps) SupportsDeleteAll() bool {
	return false
}

func (o *roleOps) Equal(a, b types.Resource153) bool {
	return EqualLegacyResource[types.Role](a, b)
}
