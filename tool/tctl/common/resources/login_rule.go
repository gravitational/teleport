/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package resources

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/gravitational/trace"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/loginrule"
)

type loginRuleCollection struct {
	rules []*loginrulepb.LoginRule
}

func (l *loginRuleCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Priority"})
	for _, rule := range l.rules {
		t.AddRow([]string{rule.GetMetadata().GetName(), strconv.FormatInt(int64(rule.GetPriority()), 10)})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func (l *loginRuleCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(l.rules))
	for i, rule := range l.rules {
		resources[i] = loginrule.ProtoToResource(rule)
	}
	return resources
}

func loginRuleHandler() Handler {
	return Handler{
		getHandler:    getLoginRule,
		createHandler: createLoginRule,
		deleteHandler: deleteLoginRule,
		singleton:     false,
		mfaRequired:   false,
		description:   "Transforms SSO user traits during login.",
	}
}

func getLoginRule(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	loginRuleClient := client.LoginRuleClient()
	if ref.Name == "" {
		rules, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, token string) ([]*loginrulepb.LoginRule, string, error) {
			resp, err := loginRuleClient.ListLoginRules(ctx, &loginrulepb.ListLoginRulesRequest{
				PageSize:  int32(limit),
				PageToken: token,
			})
			return resp.GetLoginRules(), resp.GetNextPageToken(), trace.Wrap(err)
		}))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &loginRuleCollection{rules}, nil
	}
	rule, err := loginRuleClient.GetLoginRule(ctx, &loginrulepb.GetLoginRuleRequest{
		Name: ref.Name,
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return &loginRuleCollection{[]*loginrulepb.LoginRule{rule}}, nil
}

func createLoginRule(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	rule, err := loginrule.UnmarshalLoginRule(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}

	loginRuleClient := client.LoginRuleClient()
	if opts.Force {
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
	verb := upsertVerb(false /* we don't know if it existed before */, opts.Force /* force update */)
	fmt.Printf("login_rule %q has been %s\n", rule.GetMetadata().GetName(), verb)
	return nil
}

func deleteLoginRule(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	loginRuleClient := client.LoginRuleClient()
	_, err := loginRuleClient.DeleteLoginRule(ctx, &loginrulepb.DeleteLoginRuleRequest{
		Name: ref.Name,
	})
	if err != nil {
		return trail.FromGRPC(err)
	}
	fmt.Printf("login rule %q has been deleted\n", ref.Name)
	return nil
}
