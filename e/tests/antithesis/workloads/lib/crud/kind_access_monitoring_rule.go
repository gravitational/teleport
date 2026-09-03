package crud

import (
	"context"

	"github.com/gravitational/trace"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type accessMonitoringRuleClientGetter interface {
	AccessMonitoringRuleClient() services.AccessMonitoringRules
}

// AccessMonitoringRuleOpsForClient returns type-erased CRUD operations for access monitoring rule resources.
func AccessMonitoringRuleOpsForClient(client any) (KindResourceOps, error) {
	if client, ok := client.(services.AccessMonitoringRules); ok {
		return AccessMonitoringRuleOps(client), nil
	}
	if client, ok := client.(accessMonitoringRuleClientGetter); ok {
		return AccessMonitoringRuleOps(client.AccessMonitoringRuleClient()), nil
	}
	return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", types.KindAccessMonitoringRule)
}

type accessMonitoringRuleOps struct {
	clt services.AccessMonitoringRules
}

type accessMonitoringRuleKindOps struct {
	*accessMonitoringRuleOps
	*resourceOpsProto153[*accessmonitoringrulesv1.AccessMonitoringRule, accessmonitoringrulesv1.AccessMonitoringRule, *accessMonitoringRuleOps]
}

// AccessMonitoringRuleOps returns CRUD operations for access monitoring rule resources.
func AccessMonitoringRuleOps(client services.AccessMonitoringRules) KindResourceOps {
	ops := &accessMonitoringRuleOps{clt: client}
	return &accessMonitoringRuleKindOps{
		accessMonitoringRuleOps: ops,
		resourceOpsProto153:     &resourceOpsProto153[*accessmonitoringrulesv1.AccessMonitoringRule, accessmonitoringrulesv1.AccessMonitoringRule, *accessMonitoringRuleOps]{ops: ops},
	}
}

func (o *accessMonitoringRuleOps) Kind() string {
	return types.KindAccessMonitoringRule
}

func (o *accessMonitoringRuleOps) NewResource(name string) (types.Resource153, error) {
	return services.NewAccessMonitoringRuleWithLabels(name, nil, accessmonitoringrulesv1.AccessMonitoringRuleSpec_builder{
		Subjects:  []string{types.KindRole},
		Condition: "true",
	}.Build())
}

func (o *accessMonitoringRuleOps) Clone(resource *accessmonitoringrulesv1.AccessMonitoringRule) *accessmonitoringrulesv1.AccessMonitoringRule {
	return CloneProto(resource)
}

func (o *accessMonitoringRuleOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	created, err := o.clt.CreateAccessMonitoringRule(ctx, mustCastResource[*accessmonitoringrulesv1.AccessMonitoringRule](resource))
	return created, trace.Wrap(err)
}

func (o *accessMonitoringRuleOps) Get(ctx context.Context, name string) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	return o.clt.GetAccessMonitoringRule(ctx, name)
}

func (o *accessMonitoringRuleOps) List(ctx context.Context, pageSize int, pageToken string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error) {
	rules, next, err := o.clt.ListAccessMonitoringRules(ctx, pageSize, pageToken)
	return rules, next, trace.Wrap(err)
}

func (o *accessMonitoringRuleOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	updated, err := o.clt.UpdateAccessMonitoringRule(ctx, mustCastResource[*accessmonitoringrulesv1.AccessMonitoringRule](resource))
	return updated, trace.Wrap(err)
}

func (o *accessMonitoringRuleOps) Delete(ctx context.Context, name string) error {
	return trace.Wrap(o.clt.DeleteAccessMonitoringRule(ctx, name))
}

func (o *accessMonitoringRuleOps) DeleteAll(ctx context.Context) error {
	return trace.Wrap(o.clt.DeleteAllAccessMonitoringRules(ctx))
}

func (o *accessMonitoringRuleOps) SupportsDeleteAll() bool {
	return true
}

func (o *accessMonitoringRuleOps) Equal(a, b types.Resource153) bool {
	return EqualProtoMessage(
		mustCastResource[*accessmonitoringrulesv1.AccessMonitoringRule](a),
		mustCastResource[*accessmonitoringrulesv1.AccessMonitoringRule](b),
	)
}
