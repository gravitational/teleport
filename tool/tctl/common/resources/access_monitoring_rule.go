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
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/resourceregistry"
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
		rows = append(rows, []string{item.GetMetadata().GetName(), labels})
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

func registeredAccessMonitoringRuleSpec() resourceregistry.Spec[*accessmonitoringrulesv1pb.AccessMonitoringRule, resourceregistry.NameID] {
	return resourceregistry.MustGet[*accessmonitoringrulesv1pb.AccessMonitoringRule, resourceregistry.NameID](
		resourceregistry.Default(),
		types.KindAccessMonitoringRule,
	)
}

func getAccessMonitoringRule(ctx context.Context, client *authclient.Client, ref services.Ref, _ GetOpts) (Collection, error) {
	return getRegisteredResources(ctx, client, ref, registeredAccessMonitoringRuleSpec(), func(items []*accessmonitoringrulesv1pb.AccessMonitoringRule) Collection {
		return &accessMonitoringRuleCollection{items: items}
	})
}

func createAccessMonitoringRule(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	spec := registeredAccessMonitoringRuleSpec()
	in, err := decodeRegisteredResource(raw, spec)
	if err != nil {
		return trace.Wrap(err)
	}

	resourceClient, err := registeredClient(client, spec)
	if err != nil {
		return trace.Wrap(err)
	}

	if opts.Force {
		if _, err = upsertRegisteredResource(ctx, resourceClient, in); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("access monitoring rule %q has been created\n", in.GetMetadata().GetName())
		return nil
	}

	if _, err = resourceClient.Create(ctx, in); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("access monitoring rule %q has been created\n", in.GetMetadata().GetName())
	return nil
}

func updateAccessMonitoringRule(ctx context.Context, client *authclient.Client, raw services.UnknownResource, _ CreateOpts) error {
	spec := registeredAccessMonitoringRuleSpec()
	in, err := decodeRegisteredResource(raw, spec)
	if err != nil {
		return trace.Wrap(err)
	}
	resourceClient, err := registeredClient(client, spec)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := resourceClient.Update(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("access monitoring rule %q has been updated\n", in.GetMetadata().GetName())
	return nil
}

func deleteAccessMonitoringRule(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	resourceClient, err := registeredClient(client, registeredAccessMonitoringRuleSpec())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := resourceClient.Delete(ctx, resourceregistry.NameID(ref.Name)); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Access monitoring rule %q has been deleted\n", ref.Name)
	return nil
}
