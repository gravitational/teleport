/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

	"github.com/gravitational/trace"

	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/databaseobjectimportrule"
)

type databaseObjectImportRuleCollection struct {
	rules []*dbobjectimportrulev1.DatabaseObjectImportRule
}

// NewDatabaseObjectImportRuleCollection creates a [Collection] over the provided database object import rules.
func NewDatabaseObjectImportRuleCollection(rules []*dbobjectimportrulev1.DatabaseObjectImportRule) Collection {
	return &databaseObjectImportRuleCollection{rules: rules}
}

func (c *databaseObjectImportRuleCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.rules))
	for i, b := range c.rules {
		resources[i] = databaseobjectimportrule.ProtoToResource(b)
	}
	return resources
}

func (c *databaseObjectImportRuleCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name", "Priority", "Mapping Count", "DB Label Count"})
	for _, b := range c.rules {
		t.AddRow([]string{
			b.GetMetadata().GetName(),
			fmt.Sprintf("%v", b.GetSpec().GetPriority()),
			fmt.Sprintf("%v", len(b.GetSpec().GetMappings())),
			fmt.Sprintf("%v", len(b.GetSpec().GetDatabaseLabels())),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func databaseObjectImportRuleHandler() Handler {
	return Handler{
		getHandler:    getDatabaseObjectImportRule,
		createHandler: createDatabaseObjectImportRule,
		deleteHandler: deleteDatabaseObjectImportRule,
		singleton:     false,
		mfaRequired:   false,
		description:   "Database object import rules for automatic database object discovery.",
	}
}

func getDatabaseObjectImportRule(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	remote := client.DatabaseObjectImportRuleClient()
	if ref.Name != "" {
		rule, err := remote.GetDatabaseObjectImportRule(ctx, &dbobjectimportrulev1.GetDatabaseObjectImportRuleRequest{Name: ref.Name})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &databaseObjectImportRuleCollection{rules: []*dbobjectimportrulev1.DatabaseObjectImportRule{rule}}, nil
	}

	rules, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, token string) ([]*dbobjectimportrulev1.DatabaseObjectImportRule, string, error) {
		resp, err := remote.ListDatabaseObjectImportRules(ctx, &dbobjectimportrulev1.ListDatabaseObjectImportRulesRequest{
			PageSize:  int32(limit),
			PageToken: token,
		})
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return resp.Rules, resp.NextPageToken, nil
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &databaseObjectImportRuleCollection{rules: rules}, nil
}

func createDatabaseObjectImportRule(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	rule, err := databaseobjectimportrule.UnmarshalJSON(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	if opts.Force {
		_, err = client.DatabaseObjectImportRuleClient().UpsertDatabaseObjectImportRule(ctx, &dbobjectimportrulev1.UpsertDatabaseObjectImportRuleRequest{
			Rule: rule,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("rule %q has been created\n", rule.GetMetadata().GetName())
		return nil
	}
	_, err = client.DatabaseObjectImportRuleClient().CreateDatabaseObjectImportRule(ctx, &dbobjectimportrulev1.CreateDatabaseObjectImportRuleRequest{
		Rule: rule,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("rule %q has been created\n", rule.GetMetadata().GetName())
	return nil
}

func deleteDatabaseObjectImportRule(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if _, err := client.DatabaseObjectImportRuleClient().DeleteDatabaseObjectImportRule(ctx, &dbobjectimportrulev1.DeleteDatabaseObjectImportRuleRequest{Name: ref.Name}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Rule %q has been deleted\n", ref.Name)
	return nil
}
