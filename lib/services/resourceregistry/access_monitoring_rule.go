package resourceregistry

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/client/accessmonitoringrules"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type accessMonitoringRuleReader interface {
	GetAccessMonitoringRule(context.Context, string) (*accessmonitoringrulesv1.AccessMonitoringRule, error)
	ListAccessMonitoringRules(context.Context, int, string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error)
}

type accessMonitoringRuleClientGetter interface {
	AccessMonitoringRuleClient() services.AccessMonitoringRules
}

type accessMonitoringRulesClientGetter interface {
	AccessMonitoringRulesClient() *accessmonitoringrules.Client
}

// AccessMonitoringRuleSpec is a modern RFD-153 resource registration example.
func AccessMonitoringRuleSpec() Spec[*accessmonitoringrulesv1.AccessMonitoringRule, NameID] {
	return Spec[*accessmonitoringrulesv1.AccessMonitoringRule, NameID]{
		Kind: types.KindAccessMonitoringRule,
		New: func() *accessmonitoringrulesv1.AccessMonitoringRule {
			return accessmonitoringrulesv1.AccessMonitoringRule_builder{}.Build()
		},
		Clone: proto.CloneOf[*accessmonitoringrulesv1.AccessMonitoringRule],
		ID: func(r *accessmonitoringrulesv1.AccessMonitoringRule) NameID {
			return NameID(r.GetMetadata().GetName())
		},
		Revision: func(r *accessmonitoringrulesv1.AccessMonitoringRule) string {
			return r.GetMetadata().GetRevision()
		},
		Marshal:   services.MarshalAccessMonitoringRule,
		Unmarshal: services.UnmarshalAccessMonitoringRule,
		Validate:  services.ValidateAccessMonitoringRule,
		ReadClient: func(client any) (Reader[*accessmonitoringrulesv1.AccessMonitoringRule, NameID], error) {
			c, err := accessMonitoringRuleReaderFor(client)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return accessMonitoringRuleReadClient{client: c}, nil
		},
		Client: func(client any) (Client[*accessmonitoringrulesv1.AccessMonitoringRule, NameID], error) {
			c, err := accessMonitoringRuleClientFor(client)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return accessMonitoringRuleClient{
				accessMonitoringRuleReadClient: accessMonitoringRuleReadClient{client: c},
				client:                         c,
			}, nil
		},
	}
}

func accessMonitoringRuleReaderFor(client any) (accessMonitoringRuleReader, error) {
	if c, ok := client.(accessMonitoringRuleReader); ok {
		return c, nil
	}
	if c, ok := client.(accessMonitoringRuleClientGetter); ok {
		return c.AccessMonitoringRuleClient(), nil
	}
	if c, ok := client.(accessMonitoringRulesClientGetter); ok {
		return c.AccessMonitoringRulesClient(), nil
	}
	return nil, trace.NotImplemented("client does not provide %q read operations", types.KindAccessMonitoringRule)
}

func accessMonitoringRuleClientFor(client any) (services.AccessMonitoringRules, error) {
	if c, ok := client.(services.AccessMonitoringRules); ok {
		return c, nil
	}
	if c, ok := client.(accessMonitoringRuleClientGetter); ok {
		return c.AccessMonitoringRuleClient(), nil
	}
	if c, ok := client.(accessMonitoringRulesClientGetter); ok {
		return c.AccessMonitoringRulesClient(), nil
	}
	return nil, trace.NotImplemented("client does not provide %q CRUD", types.KindAccessMonitoringRule)
}

type accessMonitoringRuleReadClient struct {
	client accessMonitoringRuleReader
}

func (c accessMonitoringRuleReadClient) Get(ctx context.Context, id NameID) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	resource, err := c.client.GetAccessMonitoringRule(ctx, string(id))
	return resource, trace.Wrap(err)
}

func (c accessMonitoringRuleReadClient) List(ctx context.Context, page Page) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error) {
	resources, next, err := c.client.ListAccessMonitoringRules(ctx, page.Size, page.Token)
	return resources, next, trace.Wrap(err)
}

type accessMonitoringRuleClient struct {
	accessMonitoringRuleReadClient
	client services.AccessMonitoringRules
}

func (c accessMonitoringRuleClient) Create(ctx context.Context, resource *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	created, err := c.client.CreateAccessMonitoringRule(ctx, resource)
	return created, trace.Wrap(err)
}

func (c accessMonitoringRuleClient) Update(ctx context.Context, resource *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	updated, err := c.client.UpdateAccessMonitoringRule(ctx, resource)
	return updated, trace.Wrap(err)
}

func (c accessMonitoringRuleClient) Upsert(ctx context.Context, resource *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	upserted, err := c.client.UpsertAccessMonitoringRule(ctx, resource)
	return upserted, trace.Wrap(err)
}

func (c accessMonitoringRuleClient) Delete(ctx context.Context, id NameID) error {
	return trace.Wrap(c.client.DeleteAccessMonitoringRule(ctx, string(id)))
}
