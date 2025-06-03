package resource

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/loginrule"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

func (rc *ResourceCommand) createLoginRule(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	rule, err := loginrule.UnmarshalLoginRule(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}

	loginRuleClient := client.LoginRuleClient()
	if rc.IsForced() {
		_, err := loginRuleClient.UpsertLoginRule(ctx, &loginrulepb.UpsertLoginRuleRequest{
			LoginRule: rule,
		})
		if err != nil {
			return trail.FromGRPC(err)
		}
	} else {
		_, err = loginRuleClient.CreateLoginRule(ctx, &loginrulepb.CreateLoginRuleRequest{
			LoginRule: rule,
		})
		if err != nil {
			return trail.FromGRPC(err)
		}
	}
	verb := UpsertVerb(false /* we don't know if it existed before */, rc.IsForced() /* force update */)
	fmt.Printf("login_rule %q has been %s\n", rule.GetMetadata().GetName(), verb)
	return nil
}

func (rc *ResourceCommand) getLoginRule(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	loginRuleClient := client.LoginRuleClient()
	if rc.ref.Name == "" {
		fetch := func(token string) (*loginrulepb.ListLoginRulesResponse, error) {
			resp, err := loginRuleClient.ListLoginRules(ctx, &loginrulepb.ListLoginRulesRequest{
				PageToken: token,
			})
			return resp, trail.FromGRPC(err)
		}
		var rules []*loginrulepb.LoginRule
		resp, err := fetch("")
		for ; err == nil; resp, err = fetch(resp.NextPageToken) {
			rules = append(rules, resp.LoginRules...)
			if resp.NextPageToken == "" {
				break
			}
		}
		return collections.NewLoginRuleCollection(rules), nil
	}
	rule, err := loginRuleClient.GetLoginRule(ctx, &loginrulepb.GetLoginRuleRequest{
		Name: rc.ref.Name,
	})
	return collections.NewLoginRuleCollection([]*loginrulepb.LoginRule{rule}), trail.FromGRPC(err)
}
