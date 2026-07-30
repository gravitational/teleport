package crud

import (
	"context"

	"github.com/gravitational/trace"

	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/healthcheckconfig"
	"github.com/gravitational/teleport/lib/services"
)

// HealthCheckConfigOpsForClient returns type-erased CRUD operations for health check config resources.
func HealthCheckConfigOpsForClient(client any) (KindResourceOps, error) {
	c, ok := client.(services.HealthCheckConfig)
	if !ok {
		return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", types.KindHealthCheckConfig)
	}
	return HealthCheckConfigOps(c), nil
}

type healthCheckConfigOps struct {
	clt services.HealthCheckConfig
}

type healthCheckConfigKindOps struct {
	*healthCheckConfigOps
	*resourceOpsProto153[*healthcheckconfigv1.HealthCheckConfig, healthcheckconfigv1.HealthCheckConfig, *healthCheckConfigOps]
}

// HealthCheckConfigOps returns CRUD operations for health check config resources.
func HealthCheckConfigOps(client services.HealthCheckConfig) KindResourceOps {
	ops := &healthCheckConfigOps{clt: client}
	return &healthCheckConfigKindOps{
		healthCheckConfigOps: ops,
		resourceOpsProto153:  &resourceOpsProto153[*healthcheckconfigv1.HealthCheckConfig, healthcheckconfigv1.HealthCheckConfig, *healthCheckConfigOps]{ops: ops},
	}
}

func (o *healthCheckConfigOps) Kind() string {
	return types.KindHealthCheckConfig
}

func (o *healthCheckConfigOps) NewResource(name string) (types.Resource153, error) {
	return healthcheckconfig.NewHealthCheckConfig(name, &healthcheckconfigv1.HealthCheckConfigSpec{
		Match: &healthcheckconfigv1.Matcher{
			Disabled: true,
		},
	})
}

func (o *healthCheckConfigOps) Clone(resource *healthcheckconfigv1.HealthCheckConfig) *healthcheckconfigv1.HealthCheckConfig {
	return CloneProto(resource)
}

func (o *healthCheckConfigOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	created, err := o.clt.CreateHealthCheckConfig(ctx, mustCastResource[*healthcheckconfigv1.HealthCheckConfig](resource))
	return created, trace.Wrap(err)
}

func (o *healthCheckConfigOps) Get(ctx context.Context, name string) (*healthcheckconfigv1.HealthCheckConfig, error) {
	return o.clt.GetHealthCheckConfig(ctx, name)
}

func (o *healthCheckConfigOps) List(ctx context.Context, pageSize int, pageToken string) ([]*healthcheckconfigv1.HealthCheckConfig, string, error) {
	configs, next, err := o.clt.ListHealthCheckConfigs(ctx, pageSize, pageToken)
	return configs, next, trace.Wrap(err)
}

func (o *healthCheckConfigOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	updated, err := o.clt.UpdateHealthCheckConfig(ctx, mustCastResource[*healthcheckconfigv1.HealthCheckConfig](resource))
	return updated, trace.Wrap(err)
}

func (o *healthCheckConfigOps) Delete(ctx context.Context, name string) error {
	return trace.Wrap(o.clt.DeleteHealthCheckConfig(ctx, name))
}

func (o *healthCheckConfigOps) DeleteAll(ctx context.Context) error {
	return trace.NotImplemented("delete-all is not implemented for kind %q", o.Kind())
}

func (o *healthCheckConfigOps) SupportsDeleteAll() bool {
	return false
}

func (o *healthCheckConfigOps) Equal(a, b types.Resource153) bool {
	return EqualProtoMessage(
		mustCastResource[*healthcheckconfigv1.HealthCheckConfig](a),
		mustCastResource[*healthcheckconfigv1.HealthCheckConfig](b),
	)
}
