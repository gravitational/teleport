package generic

import (
	"context"
	"time"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
	"github.com/gravitational/trace"
	tfpath "github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

type roleAdapter struct{}

func (roleAdapter) GetName(role *types.RoleV6) string {
	return role.GetName()
}

func (roleAdapter) GetRevision(role *types.RoleV6) string {
	return role.GetRevision()
}

type roleClient struct {
	clt *client.Client
}

func (r *roleClient) Get(ctx context.Context, name string) (*types.RoleV6, error) {
	role, err := r.clt.GetRole(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rolev6, ok := role.(*types.RoleV6)
	if !ok {
		return nil, trace.BadParameter("expected role to be *types.RoleV6, got %T", role)
	}
	return rolev6, nil
}

func (r *roleClient) Create(ctx context.Context, role *types.RoleV6) error {
	_, err := r.clt.CreateRole(ctx, role)
	return trace.Wrap(err)
}

func (r *roleClient) Upsert(ctx context.Context, role *types.RoleV6) error {
	_, err := r.clt.UpsertRole(ctx, role)
	return trace.Wrap(err)
}

func (r *roleClient) Delete(ctx context.Context, name string) error {
	return trace.Wrap(r.clt.DeleteRole(ctx, name))
}

func NewRoleResourceType() tfsdk.ResourceType {
	return tfResourceType[types.RoleV6]{
		schema: tfschema.GenSchemaRoleV6,
		res: func(p tfsdk.Provider) (*tfResource[types.RoleV6], error) {
			provider, ok := p.(teleportProvider)
			if !ok {
				return nil, trace.BadParameter("failed type asserting the provider, this is a bug")
			}
			return &tfResource[types.RoleV6]{
				fromTerraform: tfschema.CopyRoleV6FromTerraform,
				toTerraform:   tfschema.CopyRoleV6ToTerraform,
				kind:          types.KindRole,
				tfKind:        "teleport_role",
				idPath:        tfpath.Root("metadata").AtName("name"),
				a:             roleAdapter{},
				clt:           &roleClient{clt: provider.GetClient()},
				// TODO: pass retryConfig
				retryConfig: RetryConfig{
					Base:     time.Second,
					Cap:      10 * time.Second,
					MaxTries: 10,
				},
				providerIsConfigured: provider.IsConfigured,
			}, nil
		},
	}
}
