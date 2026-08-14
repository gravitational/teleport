package crud

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// UserGroupOpsForClient returns type-erased CRUD operations for user group resources.
func UserGroupOpsForClient(client any) (KindResourceOps, error) {
	c, ok := client.(services.UserGroups)
	if !ok {
		return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", types.KindUserGroup)
	}
	return UserGroupOps(c), nil
}

type userGroupOps struct {
	clt services.UserGroups
}

type userGroupKindOps struct {
	*userGroupOps
	*resourceOpsLegacy[*userGroupOps, types.UserGroup]
}

// UserGroupOps returns CRUD operations for user group resources.
func UserGroupOps(client services.UserGroups) KindResourceOps {
	ops := &userGroupOps{clt: client}
	return &userGroupKindOps{
		userGroupOps:      ops,
		resourceOpsLegacy: &resourceOpsLegacy[*userGroupOps, types.UserGroup]{ops: ops},
	}
}

func (o *userGroupOps) Kind() string {
	return types.KindUserGroup
}

func (o *userGroupOps) NewResource(name string) (types.Resource153, error) {
	group, err := types.NewUserGroup(types.Metadata{Name: name}, types.UserGroupSpecV1{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.LegacyToResource153(group), nil
}

func (o *userGroupOps) Clone(resource types.UserGroup) types.UserGroup {
	return resource.Clone()
}

func (o *userGroupOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	group, err := unwrapLegacy[types.UserGroup](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, trace.Wrap(o.clt.CreateUserGroup(ctx, group))
}

func (o *userGroupOps) Get(ctx context.Context, name string) (types.UserGroup, error) {
	return o.clt.GetUserGroup(ctx, name)
}

func (o *userGroupOps) List(ctx context.Context, pageSize int, pageToken string) ([]types.UserGroup, string, error) {
	groups, next, err := o.clt.ListUserGroups(ctx, pageSize, pageToken)
	return groups, next, trace.Wrap(err)
}

func (o *userGroupOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	group, err := unwrapLegacy[types.UserGroup](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, trace.Wrap(o.clt.UpdateUserGroup(ctx, group))
}

func (o *userGroupOps) Delete(ctx context.Context, name string) error {
	return trace.Wrap(o.clt.DeleteUserGroup(ctx, name))
}

func (o *userGroupOps) DeleteAll(ctx context.Context) error {
	return trace.Wrap(o.clt.DeleteAllUserGroups(ctx))
}

func (o *userGroupOps) SupportsDeleteAll() bool {
	return true
}

func (o *userGroupOps) Equal(a, b types.Resource153) bool {
	return EqualLegacyResource[types.UserGroup](a, b)
}
