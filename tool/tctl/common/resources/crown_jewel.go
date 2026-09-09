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

	"github.com/gravitational/trace"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/common"
)

type crownJewelCollection struct {
	items []*crownjewelv1.CrownJewel
}

func (c *crownJewelCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.Resource153ToLegacy(resource))
	}
	return r
}

// WriteText formats the crown jewels into a table and writes them into w.
// If verbose is disabled, labels column can be truncated to fit into the console.
func (c *crownJewelCollection) WriteText(w io.Writer, verbose bool) error {
	var rows [][]string
	for _, item := range c.items {
		labels := common.FormatLabels(item.GetMetadata().GetLabels(), verbose)
		rows = append(rows, []string{item.GetMetadata().GetName(), item.GetSpec().String(), labels})
	}
	headers := []string{"Name", "Spec", "Labels"}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Labels")
	}
	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func crownJewelHandler() Handler {
	return Handler{
		getHandler:    getCrownJewel,
		createHandler: createCrownJewel,
		updateHandler: updateCrownJewel,
		deleteHandler: deleteCrownJewel,
		description:   "Identifies a critical asset for Identity Security analysis and alerting.",
	}
}

func getCrownJewel(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	jewels, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, startKey string) ([]*crownjewelv1.CrownJewel, string, error) {
		return client.CrownJewelsClient().ListCrownJewels(ctx, int64(limit), startKey)
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &crownJewelCollection{items: jewels}, nil
}

func createCrownJewel(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	crownJewel, err := services.UnmarshalCrownJewel(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.CrownJewelsClient()
	if opts.Force {
		if _, err := c.UpsertCrownJewel(ctx, crownJewel); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("crown jewel %q has been updated\n", crownJewel.GetMetadata().GetName())
	} else {
		if _, err := c.CreateCrownJewel(ctx, crownJewel); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("crown jewel %q has been created\n", crownJewel.GetMetadata().GetName())
	}

	return nil
}

func updateCrownJewel(ctx context.Context, client *authclient.Client, resource services.UnknownResource, opts CreateOpts) error {
	in, err := services.UnmarshalCrownJewel(resource.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := client.CrownJewelsClient().UpdateCrownJewel(ctx, in); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("crown jewel %q has been updated\n", in.GetMetadata().GetName())
	return nil
}

func deleteCrownJewel(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if err := client.CrownJewelsClient().DeleteCrownJewel(ctx, ref.Name); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("crown_jewel %q has been deleted\n", ref.Name)
	return nil
}
