package resourceregistry

import (
	"context"

	"github.com/gravitational/trace"

	authpb "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type roleReader interface {
	GetRole(context.Context, string) (types.Role, error)
	ListRoles(context.Context, *authpb.ListRolesRequest) (*authpb.ListRolesResponse, error)
}

// RoleSpec is a legacy types.Resource registration example.
func RoleSpec() Spec[types.Role, NameID] {
	return Spec[types.Role, NameID]{
		Kind: types.KindRole,
		New: func() types.Role {
			return &types.RoleV6{}
		},
		Clone: func(r types.Role) types.Role {
			if r == nil {
				return nil
			}
			return r.Clone()
		},
		ID: func(r types.Role) NameID {
			return NameID(r.GetName())
		},
		Revision: func(r types.Role) string {
			return r.GetRevision()
		},
		Marshal:   services.MarshalRole,
		Unmarshal: services.UnmarshalRole,
		Validate:  services.ValidateRole,
		ReadClient: func(client any) (Reader[types.Role, NameID], error) {
			c, ok := client.(roleReader)
			if !ok {
				return nil, trace.NotImplemented("client does not provide %q read operations", types.KindRole)
			}
			return roleReadClient{client: c}, nil
		},
		Client: func(client any) (Client[types.Role, NameID], error) {
			c, ok := client.(services.Access)
			if !ok {
				return nil, trace.NotImplemented("client does not provide %q CRUD", types.KindRole)
			}
			return roleClient{
				roleReadClient: roleReadClient{client: c},
				client:         c,
			}, nil
		},
	}
}

type roleReadClient struct {
	client roleReader
}

func (c roleReadClient) Get(ctx context.Context, id NameID) (types.Role, error) {
	resource, err := c.client.GetRole(ctx, string(id))
	return resource, trace.Wrap(err)
}

func (c roleReadClient) List(ctx context.Context, page Page) ([]types.Role, string, error) {
	rsp, err := c.client.ListRoles(ctx, &authpb.ListRolesRequest{
		Limit:    int32(page.Size),
		StartKey: page.Token,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	resources := make([]types.Role, 0, len(rsp.GetRoles()))
	for _, resource := range rsp.GetRoles() {
		resources = append(resources, resource)
	}
	return resources, rsp.GetNextKey(), nil
}

type roleClient struct {
	roleReadClient
	client services.Access
}

func (c roleClient) Create(ctx context.Context, resource types.Role) (types.Role, error) {
	created, err := c.client.CreateRole(ctx, resource)
	return created, trace.Wrap(err)
}

func (c roleClient) Update(ctx context.Context, resource types.Role) (types.Role, error) {
	updated, err := c.client.UpdateRole(ctx, resource)
	return updated, trace.Wrap(err)
}

func (c roleClient) Upsert(ctx context.Context, resource types.Role) (types.Role, error) {
	upserted, err := c.client.UpsertRole(ctx, resource)
	return upserted, trace.Wrap(err)
}

func (c roleClient) Delete(ctx context.Context, id NameID) error {
	return trace.Wrap(c.client.DeleteRole(ctx, string(id)))
}
