package crud

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/services"
)

type discoveryConfigClientGetter interface {
	DiscoveryConfigClient() services.DiscoveryConfigWithStatusUpdater
}

// DiscoveryConfigOpsForClient returns type-erased CRUD operations for discovery config resources.
func DiscoveryConfigOpsForClient(client any) (KindResourceOps, error) {
	if client, ok := client.(services.DiscoveryConfigs); ok {
		return DiscoveryConfigOps(client), nil
	}
	if client, ok := client.(discoveryConfigClientGetter); ok {
		return DiscoveryConfigOps(client.DiscoveryConfigClient()), nil
	}
	return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", types.KindDiscoveryConfig)
}

type discoveryConfigOps struct {
	clt services.DiscoveryConfigs
}

type discoveryConfigKindOps struct {
	*discoveryConfigOps
	*resourceOpsLegacy[*discoveryConfigOps, *discoveryconfig.DiscoveryConfig]
}

// DiscoveryConfigOps returns CRUD operations for discovery config resources.
func DiscoveryConfigOps(client services.DiscoveryConfigs) KindResourceOps {
	ops := &discoveryConfigOps{clt: client}
	return &discoveryConfigKindOps{
		discoveryConfigOps: ops,
		resourceOpsLegacy:  &resourceOpsLegacy[*discoveryConfigOps, *discoveryconfig.DiscoveryConfig]{ops: ops},
	}
}

func (o *discoveryConfigOps) Kind() string {
	return types.KindDiscoveryConfig
}

func (o *discoveryConfigOps) NewResource(name string) (types.Resource153, error) {
	config, err := discoveryconfig.NewDiscoveryConfig(header.Metadata{Name: name}, discoveryconfig.Spec{
		DiscoveryGroup: "crud-test",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.LegacyToResource153(config), nil
}

func (o *discoveryConfigOps) Clone(resource *discoveryconfig.DiscoveryConfig) *discoveryconfig.DiscoveryConfig {
	return resource.Clone()
}

func (o *discoveryConfigOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	config, err := unwrapLegacy[*discoveryconfig.DiscoveryConfig](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	created, err := o.clt.CreateDiscoveryConfig(ctx, config)
	return types.LegacyToResource153(created), trace.Wrap(err)
}

func (o *discoveryConfigOps) Get(ctx context.Context, name string) (*discoveryconfig.DiscoveryConfig, error) {
	return o.clt.GetDiscoveryConfig(ctx, name)
}

func (o *discoveryConfigOps) List(ctx context.Context, pageSize int, pageToken string) ([]*discoveryconfig.DiscoveryConfig, string, error) {
	configs, next, err := o.clt.ListDiscoveryConfigs(ctx, pageSize, pageToken)
	return configs, next, trace.Wrap(err)
}

func (o *discoveryConfigOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	config, err := unwrapLegacy[*discoveryconfig.DiscoveryConfig](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updated, err := o.clt.UpdateDiscoveryConfig(ctx, config)
	return types.LegacyToResource153(updated), trace.Wrap(err)
}

func (o *discoveryConfigOps) Delete(ctx context.Context, name string) error {
	return trace.Wrap(o.clt.DeleteDiscoveryConfig(ctx, name))
}

func (o *discoveryConfigOps) DeleteAll(ctx context.Context) error {
	return trace.Wrap(o.clt.DeleteAllDiscoveryConfigs(ctx))
}

func (o *discoveryConfigOps) SupportsDeleteAll() bool {
	return true
}

func (o *discoveryConfigOps) Equal(a, b types.Resource153) bool {
	return EqualLegacyResource[*discoveryconfig.DiscoveryConfig](a, b)
}
