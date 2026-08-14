package crud

import (
	"context"

	"github.com/gravitational/trace"

	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
)

type userClient interface {
	CreateUser(context.Context, types.User) (types.User, error)
	GetUser(context.Context, string, bool) (types.User, error)
	ListUsers(context.Context, *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)
	UpdateUser(context.Context, types.User) (types.User, error)
	DeleteUser(context.Context, string) error
}

// UserOpsForClient returns type-erased CRUD operations for user resources.
func UserOpsForClient(client any) (KindResourceOps, error) {
	c, ok := client.(userClient)
	if !ok {
		return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", types.KindUser)
	}
	return UserOps(c), nil
}

type userOps struct {
	clt userClient
}

type userKindOps struct {
	*userOps
	*resourceOpsLegacy[*userOps, types.User]
}

// UserOps returns CRUD operations for user resources.
func UserOps(client userClient) KindResourceOps {
	ops := &userOps{clt: client}
	return &userKindOps{
		userOps:           ops,
		resourceOpsLegacy: &resourceOpsLegacy[*userOps, types.User]{ops: ops},
	}
}

func (o *userOps) Kind() string {
	return types.KindUser
}

func (o *userOps) NewResource(name string) (types.Resource153, error) {
	user, err := types.NewUser(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.LegacyToResource153(user), nil
}

func (o *userOps) Clone(resource types.User) types.User {
	return resource.Clone()
}

func (o *userOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	user, err := unwrapLegacy[types.User](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	created, err := o.clt.CreateUser(ctx, user)
	return types.LegacyToResource153(created), trace.Wrap(err)
}

func (o *userOps) Get(ctx context.Context, name string) (types.User, error) {
	return o.clt.GetUser(ctx, name, false)
}

func (o *userOps) List(ctx context.Context, pageSize int, pageToken string) ([]types.User, string, error) {
	rsp, err := o.clt.ListUsers(ctx, &userspb.ListUsersRequest{
		PageSize:  int32(pageSize),
		PageToken: pageToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	users := make([]types.User, 0, len(rsp.GetUsers()))
	for _, user := range rsp.GetUsers() {
		users = append(users, user)
	}
	return users, rsp.GetNextPageToken(), nil
}

func (o *userOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	user, err := unwrapLegacy[types.User](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updated, err := o.clt.UpdateUser(ctx, user)
	return types.LegacyToResource153(updated), trace.Wrap(err)
}

func (o *userOps) Delete(ctx context.Context, name string) error {
	return trace.Wrap(o.clt.DeleteUser(ctx, name))
}

func (o *userOps) DeleteAll(ctx context.Context) error {
	return trace.NotImplemented("delete-all is not implemented for kind %q", o.Kind())
}

func (o *userOps) SupportsDeleteAll() bool {
	return false
}

func (o *userOps) Equal(a, b types.Resource153) bool {
	return EqualLegacyResource[types.User](a, b)
}
