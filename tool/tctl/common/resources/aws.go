// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resources

// import (
// 	"context"
// 	"io"

// 	"github.com/gravitational/trace"

// 	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
// 	"github.com/gravitational/teleport/api/types"
// 	"github.com/gravitational/teleport/api/utils/clientutils"
// 	"github.com/gravitational/teleport/lib/asciitable"
// 	"github.com/gravitational/teleport/lib/auth/authclient"
// 	"github.com/gravitational/teleport/lib/itertools/stream"
// 	"github.com/gravitational/teleport/lib/services"
// )

// type managedResourceCollection struct {
// 	items []*identitycenterv1.Resource
// }

// func (c *managedResourceCollection) Resources() []types.Resource {
// 	r := make([]types.Resource, 0, len(c.items))
// 	for _, resource := range c.items {
// 		r = append(r, types.Resource153ToLegacy(resource))
// 	}
// 	return r
// }

// func (c *managedResourceCollection) WriteText(w io.Writer, verbose bool) error {
// 	headers := []string{"Name", "Kind", "ID"}

// 	var rows [][]string
// 	for _, item := range c.items {
// 		rows = append(rows, []string{
// 			item.Metadata.Name,
// 			item.Spec.Kind,
// 			item.Spec.Id,
// 		})
// 	}

// 	t := asciitable.MakeTable(headers, rows...)

// 	// stable sort by name.
// 	t.SortRowsBy([]int{0}, true)
// 	_, err := t.AsBuffer().WriteTo(w)
// 	return trace.Wrap(err)
// }

// func managedResourceHandler() Handler {
// 	return Handler{
// 		getHandler:  getManagedResource,
// 		singleton:   false,
// 		mfaRequired: false,
// 		description: "AWS Identity Center managed resource that can be requested via access requests.",
// 	}
// }

// func getManagedResource(
// 	ctx context.Context,
// 	client *authclient.Client,
// 	ref services.Ref,
// 	opts GetOpts,
// ) (Collection, error) {
// 	if ref.Name != "" {
// 		resource, err := client.GetIdentityCenterResource(ctx, ref.Name)
// 		if err != nil {
// 			return nil, trace.Wrap(err)
// 		}
// 		return &managedResourceCollection{items: []*identitycenterv1.Resource{resource}}, nil
// 	}

// 	resources, err := stream.Collect(clientutils.Resources(ctx, client.ListIdentityCenterManagedResources))
// 	if err != nil {
// 		return nil, trace.Wrap(err)
// 	}

// 	return &managedResourceCollection{items: resources}, nil
// }
