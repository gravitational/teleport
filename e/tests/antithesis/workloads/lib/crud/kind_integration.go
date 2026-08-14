package crud

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// IntegrationOpsForClient returns type-erased CRUD operations for integration resources.
func IntegrationOpsForClient(client any) (KindResourceOps, error) {
	c, ok := client.(services.Integrations)
	if !ok {
		return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", types.KindIntegration)
	}
	return IntegrationOps(c), nil
}

type integrationOps struct {
	clt services.Integrations
}

type integrationKindOps struct {
	*integrationOps
	*resourceOpsLegacy[*integrationOps, types.Integration]
}

// IntegrationOps returns CRUD operations for integration resources.
func IntegrationOps(client services.Integrations) KindResourceOps {
	ops := &integrationOps{clt: client}
	return &integrationKindOps{
		integrationOps:    ops,
		resourceOpsLegacy: &resourceOpsLegacy[*integrationOps, types.Integration]{ops: ops},
	}
}

func (o *integrationOps) Kind() string {
	return types.KindIntegration
}

func (o *integrationOps) NewResource(name string) (types.Resource153, error) {
	integration, err := types.NewIntegrationAWSOIDC(types.Metadata{Name: name}, &types.AWSOIDCIntegrationSpecV1{
		RoleARN: "arn:aws:iam::123456789012:role/" + name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.LegacyToResource153(integration), nil
}

func (o *integrationOps) Clone(resource types.Integration) types.Integration {
	return resource.Clone()
}

func (o *integrationOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	integration, err := unwrapLegacy[types.Integration](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	created, err := o.clt.CreateIntegration(ctx, integration)
	return types.LegacyToResource153(created), trace.Wrap(err)
}

func (o *integrationOps) Get(ctx context.Context, name string) (types.Integration, error) {
	return o.clt.GetIntegration(ctx, name)
}

func (o *integrationOps) List(ctx context.Context, pageSize int, pageToken string) ([]types.Integration, string, error) {
	integrations, next, err := o.clt.ListIntegrations(ctx, pageSize, pageToken)
	return integrations, next, trace.Wrap(err)
}

func (o *integrationOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	integration, err := unwrapLegacy[types.Integration](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updated, err := o.clt.UpdateIntegration(ctx, integration)
	return types.LegacyToResource153(updated), trace.Wrap(err)
}

func (o *integrationOps) Delete(ctx context.Context, name string) error {
	return trace.Wrap(o.clt.DeleteIntegration(ctx, name))
}

func (o *integrationOps) DeleteAll(ctx context.Context) error {
	return trace.Wrap(o.clt.DeleteAllIntegrations(ctx))
}

func (o *integrationOps) SupportsDeleteAll() bool {
	return true
}

func (o *integrationOps) Equal(a, b types.Resource153) bool {
	// TODO(okraport): add IsEqual support to types.Integration and enable this kind.
	panic("resource kind \"integration\" has no Equal comparison configured")
}
