package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	accessmonitoringrulesv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) createAccessMonitoringRule(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	in, err := services.UnmarshalAccessMonitoringRule(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if rc.IsForced() {
		if _, err = client.AccessMonitoringRuleClient().UpsertAccessMonitoringRule(ctx, in); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("access monitoring rule %q has been created\n", in.GetMetadata().GetName())
		return nil
	}

	if _, err = client.AccessMonitoringRuleClient().CreateAccessMonitoringRule(ctx, in); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("access monitoring rule %q has been created\n", in.GetMetadata().GetName())
	return nil
}

func (rc *ResourceCommand) getAccessMonitoringRule(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {
		rule, err := client.AccessMonitoringRuleClient().GetAccessMonitoringRule(ctx, rc.ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewAccessMonitoringRuleCollection([]*accessmonitoringrulesv1pb.AccessMonitoringRule{rule}), nil
	}

	var rules []*accessmonitoringrulesv1pb.AccessMonitoringRule
	nextToken := ""
	for {
		resp, token, err := client.AccessMonitoringRuleClient().ListAccessMonitoringRules(ctx, 0, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		rules = append(rules, resp...)
		if token == "" {
			break
		}
		nextToken = token
	}
	return collections.NewAccessMonitoringRuleCollection(rules), nil
}

func (rc *ResourceCommand) updateAccessMonitoringRule(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	in, err := services.UnmarshalAccessMonitoringRule(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.AccessMonitoringRuleClient().UpdateAccessMonitoringRule(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("access monitoring rule %q has been updated\n", in.GetMetadata().GetName())
	return nil
}
