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

	accessmonitoringrulesv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/common"
)

type accessMonitoringRuleCollection struct {
	items []*accessmonitoringrulesv1pb.AccessMonitoringRule
}

func (c *accessMonitoringRuleCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.Resource153ToLegacy(resource))
	}
	return r
}

func (c *accessMonitoringRuleCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, item := range c.items {
		labels := common.FormatLabels(item.GetMetadata().GetLabels(), verbose)
		rows = append(rows, []string{item.Metadata.GetName(), labels})
	}
	headers := []string{"Name", "Labels"}
	t := asciitable.MakeTable(headers, rows...)

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func accessMonitoringRuleHandler() Handler {
	return Handler{
		getHandler:    getAccessMonitoringRule,
		createHandler: createAccessMonitoringRule,
		updateHandler: updateAccessMonitoringRule,
		deleteHandler: deleteAccessMonitoringRule,
		description:   "Configures access request notification and automatic approval. Part of Identity Governance.",
	}
}

func getAccessMonitoringRule(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name != "" {
		rule, err := client.AccessMonitoringRuleClient().GetAccessMonitoringRule(ctx, ref.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &accessMonitoringRuleCollection{items: []*accessmonitoringrulesv1pb.AccessMonitoringRule{rule}}, nil
	}

	rules, err := stream.Collect(clientutils.Resources(ctx, client.AccessMonitoringRuleClient().ListAccessMonitoringRules))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &accessMonitoringRuleCollection{items: rules}, nil
}

func createAccessMonitoringRule(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	in, err := services.UnmarshalAccessMonitoringRule(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if opts.Force {
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

func updateAccessMonitoringRule(ctx context.Context, client *authclient.Client, raw services.UnknownResource, _ CreateOpts) error {
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

func deleteAccessMonitoringRule(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.AccessMonitoringRuleClient().DeleteAccessMonitoringRule(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Access monitoring rule %q has been deleted\n", ref.Name)
	return nil
}
